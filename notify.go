package memdb

type happening struct {
	event Event
	old   interface{}
	new   interface{}
}

// Event is a type of event emitted by the class, see the On() method
type Event int

// String describes the event type
func (e Event) String() string {
	switch e {
	case Insert:
		return "Insert event"
	case Update:
		return "Update event"
	case Remove:
		return "Remove event"
	case Expiry:
		return "Expiry event"
	default:
		break
	}
	return "Unknown event"
}

const (
	// Insert Events happen when an item is inserted for the first time
	Insert Event = iota

	// Update Events happen when an existing item is replaced with an new item
	Update

	// Remove Events happen when an existing item is deleted
	Remove

	// Expiry Events happen when items are removed due to being expired
	Expiry
)

// NotifyFunc is an event receiver that gets called when events happen
type NotifyFunc func(event Event, old, new interface{})
