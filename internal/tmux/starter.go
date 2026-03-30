package tmux

import "context"

// Starter implements runner.SessionStarter using real tmux commands.
type Starter struct{}

// StartSession checks tmux availability and creates a detached session.
func (s *Starter) StartSession(ctx context.Context, sessionName string) error {
	if err := Available(); err != nil {
		return err
	}
	return NewSession(ctx, sessionName)
}
