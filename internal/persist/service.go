package persist

// Service holds information for a systemd service.
type Service struct {
    Event     string
    Shell     string
    Target    string
    Threshold string
}
