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

const SPIEGEL = process.env.SPIEGEL_ORDNER ?? "/daten/spiegel";
const GIT_SSH = `ssh -i ${process.env.DEPLOY_KEY ?? "/daten/deploy_key"} -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=${SPIEGEL}/known_hosts`;

function git(args: string[], cwd?: string): Promise<void> {
  return new Promise((ok, fehler) => {
    const p = spawn("git", args, { cwd, env: { ...process.env, GIT_SSH_COMMAND: GIT_SSH }, stdio: ["ignore", "ignore", "pipe"] });
    let err = "";
    p.stderr.on("data", (d) => (err += d));
    p.on("close", (code) => (code === 0 ? ok() : fehler(new Error(`git ${args[0]}: ${err.trim()}`))));
  });
}

async function spiegelAktualisieren(slug: string, repoGit: string, commit: string): Promise<string> {
  await mkdir(SPIEGEL, { recursive: true });
  const ziel = join(SPIEGEL, `${slug}.git`);
  try { await access(ziel); await git(["fetch", "--quiet", "--prune", "origin", "+refs/heads/*:refs/heads/*"], ziel); }
  catch { await git(["clone", "--quiet", "--bare", repoGit, ziel]); }
  await git(["lfs", "fetch", "origin", commit], ziel); // LFS-Inhalte des Basis-Commits in den Spiegel
  return ziel;
}

async function aktiveAufgabe(db: Db, agentId: number, aufgabeId: number) {
  const a = (await db.query(
    `select a.basis_commit, p.slug, p.repo_git from aufgaben a join projekte p on p.id = a.projekt_id
     join leases l on l.aufgabe_id = a.id and l.beendet is null and l.laeuft_ab > now()
     where a.id = $1 and l.agent_id = $2 and not p.gesperrt`, [aufgabeId, agentId])).rows[0];
  if (!a) throw new Abgelehnt("only for the active reservation of this task");
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

export async function snapshotSenden(db: Db, agentId: number, aufgabeId: number, res: ServerResponse) {
  const a = await aktiveAufgabe(db, agentId, aufgabeId);
  const spiegel = await spiegelAktualisieren(a.slug, a.repo_git, a.basis_commit);
  // LFS-Filter ausdrücklich aus: Der Snapshot enthält nur Zeiger (klein); die Inhalte holt der
  // Runner einzeln und cached sie. (Debian aktiviert den Filter systemweit, git archive wendet ihn an.)
  const ohneLfs = ["-c", "filter.lfs.smudge=", "-c", "filter.lfs.process=", "-c", "filter.lfs.required=false"];
  const p = spawn("git", [...ohneLfs, "archive", "--format=tar.gz", a.basis_commit], { cwd: spiegel, stdio: ["ignore", "pipe", "pipe"] });
  res.writeHead(200, { "content-type": "application/gzip", "x-basis-commit": a.basis_commit });
  p.stdout.pipe(res);
  await new Promise<void>((ok) => p.on("close", () => ok()));
}
