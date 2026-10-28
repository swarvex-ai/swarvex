// Operator command line (in the container: `node src/cli.ts <command> …`).
//   migrate
//   create-project <slug> <file.json>         project + event "vision_published" (example: infra/stack/project-example.json)
//   set-role <slug> [provider:]<login> <role>  role: visionary | lead | contributor; e.g. github:nevs84
//   set-repo <slug> <git-address>             SSH address for snapshots (deploy key, read-only)
//   set-license <slug> <code-license> <asset-license>  e.g. GPL-3.0-or-later "CC BY-SA 4.0" (logged publicly)
//   check-log [heads.jsonl|url]               verify the hash chain and compare it with published heads
//                                             (without argument: our own table; better: the file from the anchor repo)
//   publish-head                              sign the head hash now (otherwise daily)
//   reports [all]                             open (or all) reports
//   report <id> <in_review|done|rejected> <outcome>
//   block person <[provider:]login> <reason> | block project <slug> <reason>
//   unblock person <[provider:]login> <reason> | unblock project <slug> <reason>
import { readFile } from "node:fs/promises";
import { migrieren, transaktion, verbinden } from "./db.ts";
import { ketteVerifizieren, protokollieren } from "./ereignisse.ts";
import { meldungAbschliessen, meldungenListe, sperren } from "./meldungen.ts";
import { koepfeListe, kopfVeroeffentlichen, mitKoepfenVergleichen } from "./koepfe.ts";
import { schluesselLaden } from "./auth/schluessel.ts";

async function menschId(login: string): Promise<number> {
  const [anbieter, name] = login.includes(":") ? login.split(":", 2) : [null, login];
  const treffer = (await db.query("select id from menschen where login = $1 and ($2::text is null or konto_provider = $2)", [name, anbieter])).rows;
  if (treffer.length !== 1) throw new Error(treffer.length ? `login ${name} exists at several providers; use provider:login` : `unknown person ${login}`);
  return Number(treffer[0].id);
}

async function projektId(slug: string): Promise<number> {
  const p = (await db.query("select id from projekte where slug = $1", [slug])).rows[0];
  if (!p) throw new Error(`unknown project ${slug}`);
  return Number(p.id);
}

const db = verbinden(process.env.DATABASE_URL ?? "");

/** Common OSI-approved licenses (SPDX); add more when needed. */
const OSI_LIZENZEN = new Set(["MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MPL-2.0", "LGPL-2.1-or-later", "LGPL-3.0-or-later",
  "GPL-2.0-or-later", "GPL-3.0-only", "GPL-3.0-or-later", "AGPL-3.0-only", "AGPL-3.0-or-later", "EUPL-1.2", "0BSD", "Zlib", "Unlicense"]);
const FREIE_ASSET_LIZENZEN = new Set(["CC BY 4.0", "CC BY-SA 4.0", "CC0 1.0", "CC-BY-4.0", "CC-BY-SA-4.0", "CC0-1.0", "OFL-1.1"]);
const [befehl, ...args] = process.argv.slice(2);

try {
  switch (befehl) {
    case "migrate":
      console.log((await migrieren(db)).join("\n") || "nothing to do");
      break;
    case "create-project": {
      const [slug, datei] = args;
      const p = JSON.parse(await readFile(datei, "utf8"));
      // E-27: swarvex only has open projects: code under an OSI license, assets under a free license.
      if (!OSI_LIZENZEN.has(p.license_code)) throw new Error(`license_code ${p.license_code} is not an OSI license (e.g. MIT, Apache-2.0, GPL-3.0-or-later, AGPL-3.0-or-later, MPL-2.0)`);
      if (!FREIE_ASSET_LIZENZEN.has(p.license_assets)) throw new Error(`license_assets ${p.license_assets} is not a free license (e.g. CC BY 4.0, CC BY-SA 4.0, CC0 1.0)`);
      const verfahren = p.procedure ?? "standard";
      if (verfahren === "clean_room" && !p.original?.legal_basis) throw new Error("a clean-room project needs original.legal_basis (how the original was acquired and the legal basis of the analysis)");
      await transaktion(db, async (tx) => {
        const id = Number((await tx.query(
          `insert into projekte(slug, name, vision_md, vision_kurz, engine, lizenz_code, lizenz_assets, repo_url, lead_modell, ziel, verfahren, vorbild)
           values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) returning id`,
          [slug, p.name, p.vision_md, p.vision_short, p.engine, p.license_code, p.license_assets, p.repo_url, p.lead_model, p.goal, verfahren, p.original ?? null])).rows[0].id);
        await protokollieren(tx, { projekt_id: id, akteur: "system", art: "vision_veroeffentlicht", bezug: `projekt:${id}`, daten: { slug, name: p.name, lead_modell: p.lead_model, verfahren, ...(p.original ? { vorbild: p.original } : {}) } });
        console.log(`project ${slug} created (id ${id})`);
      });
      break;
    }
    case "set-role": {
      const [slug, login, rolleEn] = args;
      const rolle = ({ visionary: "visionaer", lead: "lead", contributor: "mitstreiter" } as Record<string, string>)[rolleEn];
      if (!rolle) throw new Error("role: visionary, lead or contributor");
      await transaktion(db, async (tx) => {
        const p = (await tx.query("select id from projekte where slug = $1", [slug])).rows[0];
        const [anbieter, name] = login.includes(":") ? login.split(":", 2) : [null, login];
        const treffer = (await tx.query("select id from menschen where login = $1 and ($2::text is null or konto_provider = $2)", [name, anbieter])).rows;
        if (treffer.length > 1) throw new Error(`login ${name} exists at several providers; use provider:login`);
        const m = treffer[0];
        if (!p || !m) throw new Error("unknown project or person (the person must have signed in once)");
        await tx.query("insert into projekt_rollen(projekt_id, mensch_id, rolle) values ($1,$2,$3) on conflict do nothing", [p.id, m.id, rolle]);
        await protokollieren(tx, { projekt_id: Number(p.id), akteur: "system", art: "rolle_gesetzt", bezug: `mensch:${m.id}`, daten: { rolle, login } });
        console.log(`${login} is ${rolleEn} in ${slug}`);
      });
      break;
    }
    case "set-license": {
      const [slug, code, assets] = args;
      if (!OSI_LIZENZEN.has(code)) throw new Error(`${code} is not an OSI license (e.g. MIT, Apache-2.0, GPL-3.0-or-later, AGPL-3.0-or-later, MPL-2.0)`);
      if (!FREIE_ASSET_LIZENZEN.has(assets)) throw new Error(`${assets} is not a free license (e.g. CC BY 4.0, CC BY-SA 4.0, CC0 1.0)`);
      await transaktion(db, async (tx) => {
        const p = (await tx.query("update projekte set lizenz_code = $2, lizenz_assets = $3 where slug = $1 returning id", [slug, code, assets])).rows[0];
        if (!p) throw new Error(`unknown project ${slug}`);
        await protokollieren(tx, { projekt_id: Number(p.id), akteur: "system", art: "lizenz_gesetzt", bezug: `projekt:${p.id}`, daten: { lizenz_code: code, lizenz_assets: assets } });
      });
      console.log(`${slug}: ${code} / ${assets}`);
      break;
    }
    case "set-repo": {
      const [slug, adresse] = args;
      await transaktion(db, async (tx) => {
        const p = (await tx.query("update projekte set repo_git = $2 where slug = $1 returning id", [slug, adresse])).rows[0];
        if (!p) throw new Error(`unknown project ${slug}`);
        await protokollieren(tx, { projekt_id: Number(p.id), akteur: "system", art: "repo_gesetzt", bezug: `projekt:${p.id}`, daten: { repo_git: adresse } });
        console.log(`${slug}: repo ${adresse}`);
      });
      break;
    }
    case "reports": {
      const ARTEN: Record<string, string> = { rechtsverletzung: "infringement", missbrauch: "abuse", sicherheit: "security", sonstiges: "other" };
      const ZUSTAENDE: Record<string, string> = { offen: "open", in_pruefung: "in_review", erledigt: "done", abgelehnt: "rejected" };
      for (const m of await meldungenListe(db, args[0] !== "all")) {
        console.log(`#${m.id} [${ZUSTAENDE[m.zustand] ?? m.zustand}] ${ARTEN[m.art] ?? m.art}${m.projekt ? ` · ${m.projekt}` : ""} · ${m.erstellt.toISOString().slice(0, 16)} · ${m.name || "–"} <${m.email}>${m.rechteinhaber ? " (rights holder)" : ""}`);
        if (m.bezug) console.log(`   location: ${m.bezug}`);
        console.log(`   ${m.beschreibung.replace(/\n/g, "\n   ")}`);
        if (m.ergebnis) console.log(`   outcome: ${m.ergebnis}`);
      }
      break;
    }
    case "report": {
      const [id, zustandEn, ...ergebnis] = args;
      const zustand = ({ in_review: "in_pruefung", done: "erledigt", rejected: "abgelehnt" } as Record<string, "in_pruefung" | "erledigt" | "abgelehnt">)[zustandEn];
      if (!zustand) throw new Error("state: in_review, done or rejected");
      await meldungAbschliessen(db, Number(id), zustand, ergebnis.join(" "));
      console.log(`report ${id}: ${zustandEn}`);
      break;
    }
    case "block":
    case "unblock": {
      const [art, ziel, ...grund] = args;
      const gesperrt = befehl === "block";
      if (gesperrt && !grund.length) throw new Error("please give a reason (it appears in the public log)");
      if (art === "person") await sperren(db, { mensch: await menschId(ziel) }, gesperrt, grund.join(" "));
      else if (art === "project") await sperren(db, { projekt: await projektId(ziel) }, gesperrt, grund.join(" "));
      else throw new Error("block|unblock person|project <target> <reason>");
      console.log(`${art} ${ziel} ${gesperrt ? "blocked" : "unblocked"}`);
      break;
    }
    case "check-log": {
      const e = await ketteVerifizieren(db);
      // Veröffentlichte Köpfe: JSON-Zeilen bzw. JSON-Liste mit {seq, hash}, aus Datei oder URL
      const quelle = args[0];
      const roh = !quelle ? null : /^https?:\/\//.test(quelle) ? await (await fetch(quelle)).text() : await readFile(quelle, "utf8");
      const koepfe = roh === null ? await koepfeListe(db)
        : roh.trim().startsWith("[") ? JSON.parse(roh) : roh.split("\n").filter((z) => z.trim()).map((z) => JSON.parse(z));
      const vergleich = await mitKoepfenVergleichen(db, koepfe.map((k: { seq: number; hash: string }) => ({ seq: Number(k.seq), hash: k.hash })));
      console.log(JSON.stringify({ ok: e.ok, entries: e.anzahl, head: e.kopf, error: e.fehler, heads: { source: quelle ?? "own table", checked: vergleich.geprueft, deviations: vergleich.abweichungen.map((a) => ({ seq: a.seq, published: a.veroeffentlicht, chain: a.kette })) } }, null, 2));
      process.exitCode = e.ok && vergleich.abweichungen.length === 0 ? 0 : 1;
      break;
    }
    case "publish-head": {
      const k = await kopfVeroeffentlichen(db, (await schluesselLaden(db)).signatur);
      console.log(k ? `seq ${k.seq} ${k.hash}` : "no new events since the last head");
      break;
    }
    default:
      console.error("commands: migrate | create-project | set-role | set-repo | set-license | check-log [heads] | publish-head | reports | report | block | unblock");
      process.exitCode = 2;
  }
} finally {
  await db.end();
}
