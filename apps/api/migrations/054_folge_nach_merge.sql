-- Folgeaufgaben werden erst frei, wenn ihre Vorgänger auf main sind (gemerged), nicht schon bei der Annahme. Sonst
-- reserviert ein Runner im Fenster zwischen Annahme und Merge (Merge-Zug: etwa eine bis vier Minuten) einen Stand ohne
-- die Vorgängerin, und die Folgeaufgabe geht zurück oder braucht Nacharbeit. Geht ein Merge nicht durch, kommt die
-- Vorgängerin mit Bericht zurück in den Pool (Konflikt, Zug rot); bleibt er hängen, zeigt der Fluss den Engpass
-- „merge“ (bilanz.ts). Das gilt auch über Projektgrenzen (Bedarf, E-39): Das Asset ist erst auf main, wenn es gemerged ist.
create or replace function aufgabe_blockiert(abh bigint[], eins jsonb) returns boolean language sql stable as $$
  select exists (select 1 from aufgaben v where v.id = any(abh) and v.zustand <> 'gemerged')
      or exists (select 1 from jsonb_array_elements(eins) g
                 where not exists (select 1 from aufgaben v where v.id in (select x::bigint from jsonb_array_elements_text(g) x)
                                     and v.zustand = 'gemerged'))
$$;

create or replace function aufgabe_wartet_auf(abh bigint[], eins jsonb) returns jsonb language sql stable as $$
  select coalesce((select jsonb_agg(jsonb_build_array(v.id) order by v.id) from aufgaben v where v.id = any(abh) and v.zustand <> 'gemerged'), '[]'::jsonb)
      || coalesce((select jsonb_agg(g) from jsonb_array_elements(eins) g
                   where not exists (select 1 from aufgaben v where v.id in (select x::bigint from jsonb_array_elements_text(g) x)
                                       and v.zustand = 'gemerged')), '[]'::jsonb)
$$;
