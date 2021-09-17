package motd

// Even though there's just a string here right now, this is going
// to be expanded upon to make the motd dynamic based on the server
// state.

type Motd interface {
	GetMotd() string
}

type MotdProvider struct {
	motd string
}

var _ Motd = &MotdProvider{}

func New(motd string) *MotdProvider {
	p := MotdProvider{
		motd: motd,
	}

	return &p
}

// GetMotd returns a string with the motd.
func (p *MotdProvider) GetMotd() string {
	return p.motd
}
