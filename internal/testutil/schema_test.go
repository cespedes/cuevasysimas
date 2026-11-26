package testutil

import (
	"context"
	"testing"
)

// setNull are the foreign keys that are ON DELETE SET NULL on purpose: what they hold is derived and is worked
// out again (the zone of an element of the OpenStreetMap mirror, which cmd/osmsync sets on each load), so
// deleting the thing they point at must not be stopped by them. Each one is named here, with its reason, and no
// other may be.
var setNull = map[string]bool{
	"catalogue.osm_elements.osm_elements_area_id_fkey": true,
}

// One rule for every foreign key: ON DELETE RESTRICT. Nothing cascades and nothing is set to NULL, so deleting
// something never takes anything else with it unless the code does it, explicitly. The only exceptions are
// those of setNull.
func TestEveryForeignKeyIsRestrict(t *testing.T) {
	tx := NewTx(t)
	rows, err := tx.Query(context.Background(), `
		SELECT conrelid::regclass::text, conname, confdeltype::text FROM pg_constraint
		WHERE contype = 'f' AND confdeltype <> 'r' ORDER BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, name, action string
		if err := rows.Scan(&table, &name, &action); err != nil {
			t.Fatal(err)
		}
		if setNull[table+"."+name] && action == "n" { // 'n' is SET NULL: not a cascade either.
			continue
		}
		t.Errorf("%s.%s is not ON DELETE RESTRICT", table, name)
	}
}

// Everything the catalogue keeps is in its own schema: nothing in public but goose's table.
func TestNothingIsOutsideTheCatalogueSchema(t *testing.T) {
	tx := NewTx(t)
	rows, err := tx.Query(context.Background(), `
		SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'goose_db_version'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		t.Errorf("public.%s: the tables of the catalogue are in the schema catalogue", name)
	}
}
