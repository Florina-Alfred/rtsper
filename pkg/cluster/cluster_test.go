package cluster

import "testing"

func TestOwnerDeterminism(t *testing.T) {
	c, err := NewFromCSV("a,b,c", "a")
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	topic := "mytopic"
	o1 := c.Owner(topic)
	o2 := c.Owner(topic)
	if o1 != o2 {
		t.Fatalf("owner not deterministic: %s != %s", o1, o2)
	}
}

func TestOwnerSkipsDraining(t *testing.T) {
	c, err := NewFromCSV("n1,n2,n3", "n1")
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	topic := "topicX"
	orig := c.Owner(topic)
	// mark original owner draining and ensure a different owner is chosen
	c.SetDraining(orig, true)
	after := c.Owner(topic)
	if after == orig {
		t.Fatalf("expected owner to change after draining; still %s", orig)
	}
}
