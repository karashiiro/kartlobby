package motd

type MotdProvider struct {
	motd string
}

func New(motd string) *MotdProvider {
	p := MotdProvider{
		motd: motd,
	}

	return &p
}

func (p *MotdProvider) GetMotd() string {
	return p.motd
}
