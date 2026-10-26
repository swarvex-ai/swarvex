// Verwaltung über die Kommandozeile (im Container: `node src/cli.ts <befehl> …`).
//   migrieren
//   projekt-anlegen <slug> <datei.json>      Projekt + Ereignis „vision_veroeffentlicht“
//   rolle-setzen <slug> [anbieter:]<login> <rolle>  z. B. github:nevs (ohne Anbieter: eindeutiger Login)
//   repo-setzen <slug> <git-adresse>          SSH-Adresse für Snapshots (Deploy-Key, nur lesend)
//   protokoll-pruefen                         Hash-Kette verifizieren, Kopf-Hash ausgeben
//   meldungen [alle]                          offene (bzw. alle) Meldungen anzeigen
//   meldung <id> <in_pruefung|erledigt|abgelehnt> <ergebnis>
//   sperren mensch <[anbieter:]login> <grund> | sperren projekt <slug> <grund>
//   entsperren mensch <[anbieter:]login> | entsperren projekt <slug>
import { readFile } from "node:fs/promises";
import { migrieren, transaktion, verbinden } from "./db.ts";
import { ketteVerifizieren, protokollieren } from "./ereignisse.ts";
import { meldungAbschliessen, meldungenListe, sperren } from "./meldungen.ts";

async function menschId(login: string): Promise<number> {
  const [anbieter, name] = login.includes(":") ? login.split(":", 2) : [null, login];
  const treffer = (await db.query("select id from menschen where login = $1 and ($2::text is null or konto_provider = $2)", [name, anbieter])).rows;
  if (treffer.length !== 1) throw new Error(treffer.length ? `Login ${name} gibt es bei mehreren Anbietern; bitte als anbieter:login angeben` : `Mensch ${login} unbekannt`);
  return Number(treffer[0].id);
}

async function projektId(slug: string): Promise<number> {
  const p = (await db.query("select id from projekte where slug = $1", [slug])).rows[0];
  if (!p) throw new Error(`Projekt ${slug} unbekannt`);
  return Number(p.id);
}

const db = verbinden(process.env.DATABASE_URL ?? "");

/** Häufige OSI-anerkannte Lizenzen (SPDX); weitere bei Bedarf ergänzen. */
const OSI_LIZENZEN = new Set(["MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MPL-2.0", "LGPL-2.1-or-later", "LGPL-3.0-or-later",
  "GPL-2.0-or-later", "GPL-3.0-only", "GPL-3.0-or-later", "AGPL-3.0-only", "AGPL-3.0-or-later", "EUPL-1.2", "0BSD", "Zlib", "Unlicense"]);
const FREIE_ASSET_LIZENZEN = new Set(["CC BY 4.0", "CC BY-SA 4.0", "CC0 1.0", "CC-BY-4.0", "CC-BY-SA-4.0", "CC0-1.0", "OFL-1.1"]);
const [befehl, ...args] = process.argv.slice(2);

try {
  switch (befehl) {
    case "migrieren":
      console.log((await migrieren(db)).join("\n") || "nichts zu tun");
      break;
    case "projekt-anlegen": {
      const [slug, datei] = args;
      const p = JSON.parse(await readFile(datei, "utf8"));
      // E-27: Auf swarvex gibt es nur offene Projekte, Code unter einer OSI-Lizenz, Assets unter einer freien Lizenz.
      if (!OSI_LIZENZEN.has(p.lizenz_code)) throw new Error(`lizenz_code ${p.lizenz_code} ist keine OSI-Lizenz (z. B. MIT, Apache-2.0, GPL-3.0-or-later, AGPL-3.0-or-later, MPL-2.0)`);
      if (!FREIE_ASSET_LIZENZEN.has(p.lizenz_assets)) throw new Error(`lizenz_assets ${p.lizenz_assets} ist keine freie Lizenz (z. B. CC BY 4.0, CC BY-SA 4.0, CC0 1.0)`);
      await transaktion(db, async (tx) => {
        const id = Number((await tx.query(
          `insert into projekte(slug, name, vision_md, vision_kurz, engine, lizenz_code, lizenz_assets, repo_url, lead_modell, ziel, verfahren, vorbild)
           values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) returning id`,
          [slug, p.name, p.vision_md, p.vision_kurz, p.engine, p.lizenz_code, p.lizenz_assets, p.repo_url, p.lead_modell, p.ziel, p.verfahren ?? "standard", p.vorbild ?? null])).rows[0].id);
        if (p.verfahren === "clean_room" && !p.vorbild?.rechtsgrundlage) throw new Error("Clean-Room-Projekt braucht vorbild.rechtsgrundlage (Erwerb und Rechtsgrundlage der Analyse)");
        await protokollieren(tx, { projekt_id: id, akteur: "system", art: "vision_veroeffentlicht", bezug: `projekt:${id}`, daten: { slug, name: p.name, lead_modell: p.lead_modell, verfahren: p.verfahren ?? "standard", ...(p.vorbild ? { vorbild: p.vorbild } : {}) } });
        console.log(`Projekt ${slug} angelegt (id ${id})`);
      });
      break;
    }
    case "rolle-setzen": {
      const [slug, login, rolle] = args;
      await transaktion(db, async (tx) => {
        const p = (await tx.query("select id from projekte where slug = $1", [slug])).rows[0];
        const [anbieter, name] = login.includes(":") ? login.split(":", 2) : [null, login];
        const treffer = (await tx.query("select id from menschen where login = $1 and ($2::text is null or konto_provider = $2)", [name, anbieter])).rows;
        if (treffer.length > 1) throw new Error(`Login ${name} gibt es bei mehreren Anbietern; bitte als anbieter:login angeben`);
        const m = treffer[0];
        if (!p || !m) throw new Error("Projekt oder Mensch unbekannt (der Mensch muss sich einmal angemeldet haben)");
        await tx.query("insert into projekt_rollen(projekt_id, mensch_id, rolle) values ($1,$2,$3) on conflict do nothing", [p.id, m.id, rolle]);
        await protokollieren(tx, { projekt_id: Number(p.id), akteur: "system", art: "rolle_gesetzt", bezug: `mensch:${m.id}`, daten: { rolle, login } });
        console.log(`${login} ist ${rolle} in ${slug}`);
      });
      break;
    }
    case "repo-setzen": {
      const [slug, adresse] = args;
      await transaktion(db, async (tx) => {
        const p = (await tx.query("update projekte set repo_git = $2 where slug = $1 returning id", [slug, adresse])).rows[0];
        if (!p) throw new Error(`Projekt ${slug} unbekannt`);
        await protokollieren(tx, { projekt_id: Number(p.id), akteur: "system", art: "repo_gesetzt", bezug: `projekt:${p.id}`, daten: { repo_git: adresse } });
        console.log(`${slug}: Repo ${adresse}`);
      });
      break;
    }
    case "meldungen": {
      for (const m of await meldungenListe(db, args[0] !== "alle")) {
        console.log(`#${m.id} [${m.zustand}] ${m.art}${m.projekt ? ` · ${m.projekt}` : ""} · ${m.erstellt.toISOString().slice(0, 16)} · ${m.name || "–"} <${m.email}>${m.rechteinhaber ? " (Rechteinhaber)" : ""}`);
        if (m.bezug) console.log(`   Bezug: ${m.bezug}`);
        console.log(`   ${m.beschreibung.replace(/\n/g, "\n   ")}`);
        if (m.ergebnis) console.log(`   Ergebnis: ${m.ergebnis}`);
      }
      break;
    }
    case "meldung": {
      const [id, zustand, ...ergebnis] = args;
      if (!["in_pruefung", "erledigt", "abgelehnt"].includes(zustand)) throw new Error("Zustand: in_pruefung, erledigt oder abgelehnt");
      await meldungAbschliessen(db, Number(id), zustand as "in_pruefung" | "erledigt" | "abgelehnt", ergebnis.join(" "));
      console.log(`Meldung ${id}: ${zustand}`);
      break;
    }
    case "sperren":
    case "entsperren": {
      const [art, ziel, ...grund] = args;
      const gesperrt = befehl === "sperren";
      if (gesperrt && !grund.length) throw new Error("Bitte einen Grund angeben (steht im öffentlichen Protokoll)");
      if (art === "mensch") await sperren(db, { mensch: await menschId(ziel) }, gesperrt, grund.join(" "));
      else if (art === "projekt") await sperren(db, { projekt: await projektId(ziel) }, gesperrt, grund.join(" "));
      else throw new Error("sperren mensch|projekt <ziel> <grund>");
      console.log(`${art} ${ziel} ${gesperrt ? "gesperrt" : "entsperrt"}`);
      break;
    }
    case "protokoll-pruefen": {
      const e = await ketteVerifizieren(db);
      console.log(JSON.stringify(e, null, 2));
      process.exitCode = e.ok ? 0 : 1;
      break;
    }
    default:
      console.error("Befehle: migrieren | projekt-anlegen <slug> <datei> | rolle-setzen <slug> <login> <rolle> | repo-setzen <slug> <git-adresse> | protokoll-pruefen");
      process.exitCode = 2;
  }
} finally {
  await db.end();
}
