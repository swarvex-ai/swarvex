// Speichert den Zustand des OAuth-Servers (oidc-provider) in PostgreSQL:
// Sitzungen, Grants, Codes, Device-Codes, registrierte Clients, Tokens.
import type { Db } from "../db.ts";

export function pgAdapter(db: Db) {
  return class PgAdapter {
    typ: string;
    constructor(name: string) { this.typ = name; }

    async upsert(id: string, payload: any, expiresIn?: number) {
      await db.query(
        `insert into oidc_payloads(id, typ, payload, grant_id, user_code, uid, laeuft_ab)
         values ($1,$2,$3,$4,$5,$6, case when $7::int is null then null else now() + make_interval(secs => $7) end)
         on conflict (typ, id) do update set payload = excluded.payload, grant_id = excluded.grant_id,
           user_code = excluded.user_code, uid = excluded.uid, laeuft_ab = excluded.laeuft_ab`,
        [id, this.typ, payload, payload.grantId ?? null, payload.userCode ?? null, payload.uid ?? null, expiresIn ?? null],
      );
    }

    async #einer(spalte: "id" | "user_code" | "uid", wert: string) {
      const r = await db.query(
        `select payload, verbraucht from oidc_payloads where typ = $1 and ${spalte} = $2 and (laeuft_ab is null or laeuft_ab > now())`,
        [this.typ, wert],
      );
      const z = r.rows[0];
      if (!z) return undefined;
      return z.verbraucht ? { ...z.payload, consumed: Math.floor(new Date(z.verbraucht).getTime() / 1000) } : z.payload;
    }

    find(id: string) { return this.#einer("id", id); }
    findByUserCode(code: string) { return this.#einer("user_code", code); }
    findByUid(uid: string) { return this.#einer("uid", uid); }
    async consume(id: string) { await db.query("update oidc_payloads set verbraucht = now() where typ = $1 and id = $2", [this.typ, id]); }
    async destroy(id: string) { await db.query("delete from oidc_payloads where typ = $1 and id = $2", [this.typ, id]); }
    async revokeByGrantId(grantId: string) { await db.query("delete from oidc_payloads where grant_id = $1", [grantId]); }
  };
}
