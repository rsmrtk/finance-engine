package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/appleauth"
	"github.com/rsmrtk/finance-engine/pkg/jwt"
)

const defaultBaseCurrency = "UAH"

type Service struct {
	users    *repository.UserRepository
	verifier *appleauth.Verifier
	jwt      jwt.JWT
}

func New(users *repository.UserRepository, verifier *appleauth.Verifier, jwtManager jwt.JWT) *Service {
	return &Service{users: users, verifier: verifier, jwt: jwtManager}
}

type SignInResult struct {
	AccessToken string
	User        repository.User
}

// SignInWithApple verifies the identity token from the device, then finds
// or creates a user keyed by Apple's stable subject id. This is what makes
// the user's data survive app reinstalls: it's tied to the Apple account,
// never to the device or the app binary.
func (s *Service) SignInWithApple(ctx context.Context, identityToken string) (SignInResult, error) {
	sub, email, err := s.verifier.Verify(identityToken)
	if err != nil {
		return SignInResult{}, fmt.Errorf("verify apple identity token: %w", err)
	}

	user, err := s.users.GetByAppleSub(ctx, sub)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return SignInResult{}, fmt.Errorf("lookup user: %w", err)
		}
		user, err = s.users.Create(ctx, sub, email, defaultBaseCurrency)
		if err != nil {
			return SignInResult{}, fmt.Errorf("create user: %w", err)
		}
	}

	token, err := s.jwt.Generate(user.ID)
	if err != nil {
		return SignInResult{}, fmt.Errorf("generate access token: %w", err)
	}

	return SignInResult{AccessToken: token, User: user}, nil
}
