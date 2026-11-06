package command

import "testing"

func noop() {}

func TestModifiedAfterUndoAndNewChange(t *testing.T) {
	s := NewStorage()
	s.SetStack("map")
	s.Push(Make("A", noop, noop))
	s.Push(Make("B", noop, noop))
	s.ForceBalance("map") // Saved.
	if s.IsModified("map") {
		t.Fatal("saved map must not be modified")
	}

	s.Undo()
	if !s.IsModified("map") {
		t.Fatal("undo after save must make the map modified")
	}
	s.Redo()
	if s.IsModified("map") {
		t.Fatal("redo returns the saved state")
	}

	s.Undo()
	s.Push(Make("C", noop, noop)) // A different change instead of B.
	if !s.IsModified("map") {
		t.Fatal("save -> undo -> new change must be modified")
	}
}
