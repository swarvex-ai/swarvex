// PostgreSQL-Zugang: ein Pool, Transaktionen, Migrationen aus ../migrations.
import pg from "pg";
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";

// bigint (int8) als Zahl statt Text: IDs und Zähler bleiben weit unter 2^53. So liefern
// API und MCP-Werkzeuge echte Zahlen, die Clients in anderen Sprachen erwarten.
pg.types.setTypeParser(20, (wert) => Number(wert));

export type Db = pg.Pool;
export type Tx = pg.PoolClient;

export function verbinden(url: string): Db {
  const pool = new pg.Pool({ connectionString: url, max: 10 });
  // Bricht die Datenbank eine ruhende Verbindung ab (Neustart, Herunterfahren), meldet der Pool das als Ereignis;
  // ohne Behandlung beendet es den ganzen Prozess (13.11.2026, Neustart des Servers). Die nächste Anfrage verbindet neu.
  pool.on("error", (e) => console.error("database connection lost:", e.message));
  return pool;
}

export async function transaktion<T>(db: Db, arbeit: (tx: Tx) => Promise<T>): Promise<T> {
  const tx = await db.connect();
  try {
    await tx.query("begin");
    const ergebnis = await arbeit(tx);
    await tx.query("commit");
    return ergebnis;
  } catch (fehler) {
    await tx.query("rollback");
    throw fehler;
  } finally {
    tx.release();
  }
}

/** Spielt alle noch nicht angewandten Migrationen der Reihe nach ein. */
export async function migrieren(db: Db): Promise<string[]> {
  await db.query("create table if not exists migrationen (name text primary key, angewandt timestamptz not null default now())");
  const ordner = join(import.meta.dirname, "..", "migrations");
  const dateien = (await readdir(ordner)).filter((d) => d.endsWith(".sql")).sort();
  const fertig = new Set((await db.query("select name from migrationen")).rows.map((r) => r.name));
  const neu: string[] = [];
  for (const datei of dateien) {
    if (fertig.has(datei)) continue;
    const sql = await readFile(join(ordner, datei), "utf8");
    await transaktion(db, async (tx) => {
      await tx.query(sql);
      await tx.query("insert into migrationen(name) values ($1)", [datei]);
    });
    neu.push(datei);
  }
  return neu;
}
