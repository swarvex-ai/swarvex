// Einstieg des Containers spendenwerk (infra/stack/compose.yaml): Migrationen macht die API, hier nur die Schleife.
import { verbinden } from "./db.ts";
import { laufen } from "./spendenwerk.ts";

const db = verbinden(process.env.DATABASE_URL!);
const stopp = new AbortController();
for (const s of ["SIGTERM", "SIGINT"] as const) process.on(s, () => stopp.abort());
console.log("spendenwerk: running");
await laufen(db, stopp.signal);
await db.end();
