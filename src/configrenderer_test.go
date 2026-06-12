package main

import "testing"

func strPtr(v string) *string {
	return &v
}

func TestReserveSpectatorSlots(t *testing.T) {
	tests := []struct {
		name          string
		entryCount    int
		capacity      int
		spectator     bool
		wantNormal    int
		wantTotal     int
		wantErr       bool
	}{
		{name: "disabled unchanged", entryCount: 10, capacity: 4, spectator: false, wantNormal: 4, wantTotal: 4},
		{name: "enabled reserves one slot", entryCount: 10, capacity: 4, spectator: true, wantNormal: 3, wantTotal: 4},
		{name: "enabled keeps smaller grid", entryCount: 2, capacity: 8, spectator: true, wantNormal: 2, wantTotal: 3},
		{name: "enabled needs capacity", entryCount: 10, capacity: 1, spectator: true, wantErr: true},
		{name: "enabled needs race slot", entryCount: 0, capacity: 4, spectator: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNormal, gotTotal, err := reserveSpectatorSlots(tt.entryCount, tt.capacity, tt.spectator)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if gotNormal != tt.wantNormal || gotTotal != tt.wantTotal {
				t.Fatalf("got normal=%d total=%d, want normal=%d total=%d", gotNormal, gotTotal, tt.wantNormal, tt.wantTotal)
			}
		})
	}
}

func TestEntryListDataAppendsSpectator(t *testing.T) {
	class := UserClass{Entries: []UserClassEntry{
		{CacheCarKey: strPtr("ks_mazda_mx5_cup"), SkinKey: strPtr("red")},
		{CacheCarKey: strPtr("ks_mazda_mx5_cup"), SkinKey: strPtr("blue")},
	}}
	instance := ServerInstance{
		SpectatorEnabled: intPtr(1),
		SpectatorName:    strPtr("Broadcast"),
		SpectatorGuid:    strPtr("76561198000000000"),
		SpectatorCarKey:  strPtr("ks_audi_a1s1"),
		SpectatorSkinKey: strPtr("white"),
	}

	data, serverClass := entryListData(class, instance)
	if len(data.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(data.Entries))
	}
	if len(serverClass.Entries) != 3 {
		t.Fatalf("expected server CARS list to include spectator car, got %d", len(serverClass.Entries))
	}
	last := data.Entries[2]
	if last.SpectatorMode != 1 || last.CacheCarKey != "ks_audi_a1s1" || last.DriverName != "Broadcast" || last.Guid == "" {
		t.Fatalf("spectator entry not populated: %+v", last)
	}
	if data.Entries[0].SpectatorMode != 0 {
		t.Fatalf("normal entry should not be spectator: %+v", data.Entries[0])
	}
}

func TestEntryListDataDisabledUnchanged(t *testing.T) {
	class := UserClass{Entries: []UserClassEntry{
		{CacheCarKey: strPtr("ks_mazda_mx5_cup"), SkinKey: strPtr("red")},
	}}

	data, serverClass := entryListData(class, ServerInstance{})
	if len(data.Entries) != 1 || len(serverClass.Entries) != 1 {
		t.Fatalf("spectator disabled should not append entries: data=%d server=%d", len(data.Entries), len(serverClass.Entries))
	}
	if data.Entries[0].SpectatorMode != 0 {
		t.Fatalf("normal entry should stay non-spectator: %+v", data.Entries[0])
	}
}
