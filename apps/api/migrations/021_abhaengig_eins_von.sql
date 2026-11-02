-- Varianten-Paare (eine wird angenommen, die andere abgelehnt oder geschlossen): Gruppen, von denen mindestens eine Aufgabe
-- angenommen sein muss. aufgabe_blockiert() ist die eine Regel für Angebot, Reservierung und Anzeige.
alter table aufgaben add column abhaengig_eins_von jsonb not null default '[]';

create function aufgabe_blockiert(abh bigint[], eins jsonb) returns boolean language sql stable as $$
  select exists (select 1 from aufgaben v where v.id = any(abh) and v.zustand not in ('angenommen', 'gemerged'))
      or exists (select 1 from jsonb_array_elements(eins) g
                 where not exists (select 1 from aufgaben v where v.id in (select x::bigint from jsonb_array_elements_text(g) x)
                                     and v.zustand in ('angenommen', 'gemerged')))
$$;

-- Was eine Aufgabe noch braucht, als Liste von Alternativen: [[12], [45, 46]] heißt „#12 und (#45 oder #46)“.
create function aufgabe_wartet_auf(abh bigint[], eins jsonb) returns jsonb language sql stable as $$
  select coalesce((select jsonb_agg(jsonb_build_array(v.id) order by v.id) from aufgaben v where v.id = any(abh) and v.zustand not in ('angenommen', 'gemerged')), '[]'::jsonb)
      || coalesce((select jsonb_agg(g) from jsonb_array_elements(eins) g
                   where not exists (select 1 from aufgaben v where v.id in (select x::bigint from jsonb_array_elements_text(g) x)
                                       and v.zustand in ('angenommen', 'gemerged'))), '[]'::jsonb)
$$;
