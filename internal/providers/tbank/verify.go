package tbank

func (p *Provider) VerifyWebhook(payload map[string]string, token string) bool {
	expectedToken := p.generateToken(payload)
	return expectedToken == token
}
