package main

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// Die Pfade eines Beitrags gehen wie in der Prüfstraße als SWARVEX_CHANGED an den Test (eigener Lauf, Gegenprüfung).
func TestPatchPfade(t *testing.T) {
	patch := "diff --git a/src/sofa.ts b/src/sofa.ts\nnew file mode 100644\n--- /dev/null\n+++ b/src/sofa.ts\n@@ -0,0 +1 @@\n+x\n" +
		"diff --git a/src/index.ts b/src/index.ts\n--- a/src/index.ts\n+++ b/src/index.ts\n@@ -1 +1,2 @@\n a\n+b\n" +
		"diff --git a/alt.md b/neu.md\nsimilarity index 90%\nrename from alt.md\nrename to neu.md\n"
	if got, want := patchPfade(patch), []string{"src/sofa.ts", "src/index.ts", "alt.md", "neu.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("patchPfade = %q, want %q", got, want)
	}
	if patchPfade("") != nil {
		t.Error("empty patch: no paths")
	}
}

// swarvex status zeigt die eigenen Zeilen des Runners, nicht die von Podman und dem Proxy.
func TestLetzteMeldungen(t *testing.T) {
	journal := "No matching task. Next attempt in 5m0s.\n" +
		"conmon 4ed90a16b7f4542af8ec <nwarn>: Failed to open cgroups file: /sys/fs/cgroup/memory.events\n" +
		"2026-11-09 09:35:44.846909755 +0100 CET m=+0.056163216 container died 4ed90a16 (image=localhost/swarvex-proxy:latest)\n" +
		`{"type":"quota","windows":[]}` + "\n\n" +
		"error: sandbox: agent exited with an error: exit status 1\n" +
		"Leftover mode: waiting until Tue 10 Nov 01:00.\n"
	got := letzteMeldungen(journal, 5)
	want := []string{"No matching task. Next attempt in 5m0s.", "error: sandbox: agent exited with an error: exit status 1", "Leftover mode: waiting until Tue 10 Nov 01:00."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("letzteMeldungen = %q", got)
	}
	if got := letzteMeldungen(journal, 1); len(got) != 1 || got[0] != want[2] {
		t.Errorf("last one only: %q", got)
	}
}

func TestWartenMerken(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	jetzt := time.Now()
	if _, ok := wartenLesen(jetzt); ok {
		t.Fatal("nothing saved yet")
	}
	wartenMerken("leftover mode: night share from 01:00", jetzt.Add(time.Hour))
	if w, ok := wartenLesen(jetzt); !ok || w.Grund != "leftover mode: night share from 01:00" {
		t.Errorf("wartenLesen = %+v, %v", w, ok)
	}
	if _, ok := wartenLesen(jetzt.Add(2 * time.Hour)); ok {
		t.Error("a wait that is over is not shown")
	}
	wartenMerken("", time.Time{})
	if _, ok := wartenLesen(jetzt); ok {
		t.Error("a running cycle clears the wait")
	}
}

// Wartet der Runner auf Restkontingent oder Tagesbudget, prüft er in der Zeit fremde Einreichungen gegen: nach einer
// Gegenprüfung gleich die nächste, sonst nach der Pause wieder; außerhalb der Zeitfenster gar nicht.
func TestGegenpruefenBisZumWartende(t *testing.T) {
	n := time.Now().Add(300 * time.Millisecond)
	laeufe, pausen := 0, 0
	gegenpruefenBis(context.Background(), nil, n, time.Minute, func() bool { laeufe++; return laeufe <= 2 },
		func(d time.Duration) { pausen++; time.Sleep(min(d, 40*time.Millisecond)) })
	if laeufe < 3 || pausen == 0 || time.Now().Before(n) {
		t.Errorf("runs=%d pauses=%d, ended before the wait was over: %v", laeufe, pausen, time.Now().Before(n))
	}
	// Zeitfenster zu: keine Gegenprüfung
	ab := time.Now().Add(2 * time.Hour)
	zu, err := zeitfensterLesen(fmt.Sprintf("daily %02d:00-%02d:30", ab.Hour(), ab.Hour()))
	if err != nil {
		t.Fatal(err)
	}
	laeufe = 0
	gegenpruefenBis(context.Background(), zu, time.Now().Add(100*time.Millisecond), time.Minute, func() bool { laeufe++; return true },
		func(d time.Duration) { time.Sleep(min(d, 40*time.Millisecond)) })
	if laeufe != 0 {
		t.Errorf("cross-checked outside the time windows: %d", laeufe)
	}
}

// Nach einer Rückgabe gleich die nächste Aufgabe; erst Fehler in Folge machen die Pause länger.
func TestFehlerPause(t *testing.T) {
	if fehlerPause(1) != 15*time.Second || fehlerPause(2) != time.Minute || fehlerPause(3) != 5*time.Minute || fehlerPause(9) != 5*time.Minute {
		t.Errorf("fehlerPause: %v %v %v", fehlerPause(1), fehlerPause(2), fehlerPause(3))
	}
}
