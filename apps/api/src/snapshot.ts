// Projekt-Snapshot für den Runner: Mitstreiter brauchen keine Git-Zugangsdaten.
// Die Plattform hält einen Bare-Spiegel (Deploy-Key, nur lesend) und liefert den Stand des
// Basis-Commits als tar.gz – nur an den Agenten, der die Aufgabe gerade reserviert hat.
// Git-LFS-Dateien stehen darin als Zeiger; ihren Inhalt holt der Runner einzeln über
// /aufgaben/<id>/lfs/<oid> und speichert ihn nach Prüfsumme (jede Datei nur einmal laden).
import { spawn } from "node:child_process";
import { mkdir, access } from "node:fs/promises";
import { createReadStream } from "node:fs";
import { join } from "node:path";
import type { ServerResponse } from "node:http";
import type { Db } from "./db.ts";
import { Abgelehnt } from "./aufgaben.ts";
import { gegenpruefungAktiv } from "./gegenpruefung.ts";
import { gitTokenUmgebung, repoToken } from "./github.ts";

const SPIEGEL = process.env.MIRROR_DIR ?? "/data/mirror";
const GIT_SSH = `ssh -i ${process.env.DEPLOY_KEY ?? "/data/deploy_key"} -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=${SPIEGEL}/known_hosts`;

function git(args: string[], cwd?: string, umgebung: Record<string, string> = {}): Promise<void> {
  return new Promise((ok, fehler) => {
    const p = spawn("git", args, { cwd, env: { ...process.env, GIT_SSH_COMMAND: GIT_SSH, ...umgebung }, stdio: ["ignore", "ignore", "pipe"] });
    let err = "";
    p.stderr.on("data", (d) => (err += d));
    p.on("close", (code) => (code === 0 ? ok() : fehler(new Error(`git ${args[0]}: ${err.trim()}`))));
  });
}

async function spiegelAktualisieren(slug: string, repoGit: string, commit: string, installation: number | null): Promise<string> {
  await mkdir(SPIEGEL, { recursive: true });
  const ziel = join(SPIEGEL, `${slug}.git`);
  // Über die GitHub App: lesender Token nur für dieses Repo; sonst der Deploy-Key (SSH)
  const u = installation ? gitTokenUmgebung(await repoToken(installation, repoGit)) : {};
  try { await access(ziel); await git(["remote", "set-url", "origin", repoGit], ziel); await git(["fetch", "--quiet", "--prune", "origin", "+refs/heads/*:refs/heads/*"], ziel, u); }
  catch { await git(["clone", "--quiet", "--bare", repoGit, ziel], undefined, u); }
  await git(["lfs", "fetch", "origin", commit], ziel, u); // LFS-Inhalte des Basis-Commits in den Spiegel
  return ziel;
}

async function aktiveAufgabe(db: Db, agentId: number, aufgabeId: number) {
  // Zugriff hat, wer die Aufgabe reserviert hat oder ihre Einreichung gerade gegenprüft.
  const gegen = await gegenpruefungAktiv(db, agentId, aufgabeId);
  const a = (await db.query(
    `select a.basis_commit, a.raum, p.slug, p.repo_git, p.github_installation from aufgaben a join projekte p on p.id = a.projekt_id
     where a.id = $1 and not p.gesperrt and ($3 or exists (select 1 from leases l where l.aufgabe_id = a.id
       and l.beendet is null and l.laeuft_ab > now() and l.agent_id = $2))`, [aufgabeId, agentId, gegen])).rows[0];
  if (!a) throw new Abgelehnt("only for the active reservation or cross-check of this task");
  if (!a.repo_git || !a.basis_commit) throw new Abgelehnt("project without Git address or task without base commit");
  return a;
}

/** Ein LFS-Objekt (nach SHA-256) aus dem Spiegel – nur für den Inhaber der Reservierung. */
export async function lfsObjektSenden(db: Db, agentId: number, aufgabeId: number, oid: string, res: ServerResponse) {
  if (!/^[0-9a-f]{64}$/.test(oid)) throw new Abgelehnt("invalid object ID");
  const a = await aktiveAufgabe(db, agentId, aufgabeId);
  const pfad = join(SPIEGEL, `${a.slug}.git`, "lfs", "objects", oid.slice(0, 2), oid.slice(2, 4), oid);
  try { await access(pfad); } catch { throw new Abgelehnt("LFS object not in the mirror"); }
  res.writeHead(200, { "content-type": "application/octet-stream" });
  createReadStream(pfad).pipe(res);
  await new Promise<void>((ok) => res.on("close", () => ok()));
}

/** Clean Room: Wer baut, bekommt nur die freigegebene Spezifikation, nicht die Entwürfe der Analyse, ihre
 *  Prüfrunden und offenen Fragen (lead/templates/clean-room-specification.md). Pfadangaben für git archive. */
export const NICHT_FUER_BAU = ["specification/drafts", "specification/checks", "specification/unknowns.md"];
export const snapshotPfade = (raum: string | null) => (raum === "bau" ? ["--", ".", ...NICHT_FUER_BAU.map((x) => `:(exclude)${x}`)] : []);

export async function snapshotSenden(db: Db, agentId: number, aufgabeId: number, res: ServerResponse) {
  const a = await aktiveAufgabe(db, agentId, aufgabeId);
  const spiegel = await spiegelAktualisieren(a.slug, a.repo_git, a.basis_commit, a.github_installation ? Number(a.github_installation) : null);
  // LFS-Filter ausdrücklich aus: Der Snapshot enthält nur Zeiger (klein); die Inhalte holt der
  // Runner einzeln und cached sie. (Debian aktiviert den Filter systemweit, git archive wendet ihn an.)
  const ohneLfs = ["-c", "filter.lfs.smudge=", "-c", "filter.lfs.process=", "-c", "filter.lfs.required=false"];
  const p = spawn("git", [...ohneLfs, "archive", "--format=tar.gz", a.basis_commit, ...snapshotPfade(a.raum)], { cwd: spiegel, stdio: ["ignore", "pipe", "pipe"] });
  res.writeHead(200, { "content-type": "application/gzip", "x-basis-commit": a.basis_commit });
  p.stdout.pipe(res);
  await new Promise<void>((ok) => p.on("close", () => ok()));
}
