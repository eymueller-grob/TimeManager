package tracker

import (
	"fmt"
	"time"
)

func SelectExportBookings(d State, from, to string, loc *time.Location, excludeExported bool) (State, []Booking, error) {
	start, end, err := DateRange(from, to, loc)
	if err != nil {
		return State{}, nil, err
	}
	selected := clone(d)
	selected.Bookings = []Booking{}
	eligible := []Booking{}
	for _, booking := range d.Bookings {
		if (excludeExported && booking.Exported) || !booking.Start.Before(end) || !booking.End.After(start) {
			continue
		}
		selected.Bookings = append(selected.Bookings, booking)
		if !booking.Exported && !booking.Start.Before(start) && !booking.End.After(end) {
			eligible = append(eligible, booking)
		}
	}
	return selected, eligible, nil
}

func (s *Store) MarkBookingsExported(expected []Booking) error {
	return s.change(func(d *State) error {
		indices := make(map[string]int, len(d.Bookings))
		for i, booking := range d.Bookings {
			indices[booking.ID] = i
		}
		for _, booking := range expected {
			index, ok := indices[booking.ID]
			if !ok || d.Bookings[index] != booking {
				return fmt.Errorf("booking %s changed during export; no exported statuses were updated", booking.ID)
			}
		}
		for _, booking := range expected {
			d.Bookings[indices[booking.ID]].Exported = true
		}
		return nil
	})
}
