package testutil

import "testing"

func TestDB_Ping(t *testing.T) {
	db := DBForTest(t)
	defer db.Close()

	var n int
	if err := db.QueryRow("SELECT 1").Scan(&n); err != nil {
		t.Fatal("SELECT 1 failed:", err)
	}
	if n != 1 {
		t.Errorf("SELECT 1 got %d, want 1", n)
	}
}
