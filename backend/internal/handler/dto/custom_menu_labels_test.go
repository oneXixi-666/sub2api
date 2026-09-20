package dto

import "testing"

func TestNormalizeCustomMenuItemLabels(t *testing.T) {
	item := CustomMenuItem{
		Label: "  Help Center  ",
		Labels: map[string]string{
			"FR": "  Centre d’aide  ",
			"de": "Hilfe",
			"zh": "   ",
		},
	}
	NormalizeCustomMenuItemLabels(&item)
	if item.Label != "Help Center" {
		t.Fatalf("label = %q, want Help Center", item.Label)
	}
	if len(item.Labels) != 1 || item.Labels["fr"] != "Centre d’aide" {
		t.Fatalf("labels = %#v", item.Labels)
	}
}

func TestNormalizeCustomMenuItemLabelsFillsDefault(t *testing.T) {
	item := CustomMenuItem{
		Labels: map[string]string{"ru": "Справка"},
	}
	NormalizeCustomMenuItemLabels(&item)
	if item.Label != "Справка" {
		t.Fatalf("label = %q, want Справка", item.Label)
	}
}
