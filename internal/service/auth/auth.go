package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/appleauth"
	"github.com/rsmrtk/finance-engine/pkg/googleauth"
	"github.com/rsmrtk/finance-engine/pkg/jwt"
	"github.com/rsmrtk/finance-engine/pkg/passwordhash"
)

const defaultBaseCurrency = "UAH"

type Service struct {
	users          *repository.UserRepository
	verifier       *appleauth.Verifier
	googleVerifier *googleauth.Verifier
	jwt            jwt.JWT
	devMode        bool
}

func New(users *repository.UserRepository, verifier *appleauth.Verifier, googleVerifier *googleauth.Verifier, jwtManager jwt.JWT, devMode bool) *Service {
	return &Service{users: users, verifier: verifier, googleVerifier: googleVerifier, jwt: jwtManager, devMode: devMode}
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
	return s.findOrCreateAndIssueToken(ctx, sub, email)
}

// DevSignIn is a Sign in with Apple bypass for local testing: Apple requires
// a paid Developer account before the real capability works at all, so this
// lets the rest of the stack (gRPC, Postgres, sync) be exercised without
// one. Refuses to run unless devMode was explicitly enabled.
func (s *Service) DevSignIn(ctx context.Context, deviceID string) (SignInResult, error) {
	if !s.devMode {
		return SignInResult{}, fmt.Errorf("dev sign-in is disabled")
	}
	if deviceID == "" {
		return SignInResult{}, fmt.Errorf("device_id is required")
	}
	sub := "dev:" + deviceID
	return s.findOrCreateAndIssueToken(ctx, sub, "")
}

// SignUpWithEmail registers a new user with an email+password. Fails if the
// email is already taken (by any login method — email, Google, or Apple).
func (s *Service) SignUpWithEmail(ctx context.Context, email, password string) (SignInResult, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return SignInResult{}, fmt.Errorf("email and password are required")
	}
	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		return SignInResult{}, fmt.Errorf("an account with this email already exists")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SignInResult{}, fmt.Errorf("lookup user: %w", err)
	}

	hash, err := passwordhash.Hash(password)
	if err != nil {
		return SignInResult{}, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.users.CreateWithEmail(ctx, email, hash, defaultBaseCurrency)
	if err != nil {
		return SignInResult{}, fmt.Errorf("create user: %w", err)
	}

	token, err := s.jwt.Generate(user.ID)
	if err != nil {
		return SignInResult{}, fmt.Errorf("generate access token: %w", err)
	}
	return SignInResult{AccessToken: token, User: user}, nil
}

// SignInWithEmail never reveals whether the email or the password was
// wrong — both failure modes return the same generic error.
func (s *Service) SignInWithEmail(ctx context.Context, email, password string) (SignInResult, error) {
	const invalidCredentials = "invalid email or password"

	user, err := s.users.GetByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SignInResult{}, fmt.Errorf(invalidCredentials)
		}
		return SignInResult{}, fmt.Errorf("lookup user: %w", err)
	}
	if user.PasswordHash == "" {
		// This account was created via Apple/Google and has no password set.
		return SignInResult{}, fmt.Errorf(invalidCredentials)
	}
	if err := passwordhash.Verify(user.PasswordHash, password); err != nil {
		return SignInResult{}, fmt.Errorf(invalidCredentials)
	}

	token, err := s.jwt.Generate(user.ID)
	if err != nil {
		return SignInResult{}, fmt.Errorf("generate access token: %w", err)
	}
	return SignInResult{AccessToken: token, User: user}, nil
}

// SignInWithGoogle verifies the ID token from Google Identity Services,
// then finds or creates a user keyed by Google's stable subject id —
// same shape as SignInWithApple, just a different provider.
func (s *Service) SignInWithGoogle(ctx context.Context, idToken string) (SignInResult, error) {
	sub, email, err := s.googleVerifier.Verify(idToken)
	if err != nil {
		return SignInResult{}, fmt.Errorf("verify google id token: %w", err)
	}

	user, err := s.users.GetByGoogleSub(ctx, sub)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return SignInResult{}, fmt.Errorf("lookup user: %w", err)
		}
		user, err = s.users.CreateWithGoogle(ctx, sub, email, defaultBaseCurrency)
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

// Me looks up the currently authenticated user — used by the web's
// GET /api/auth/me so the frontend can tell "still logged in" apart from
// "logged out" after a page reload, since the access token lives in an
// httpOnly cookie JS can't read directly.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (repository.User, error) {
	return s.users.GetByID(ctx, userID)
}

var validThemes = map[string]bool{"system": true, "light": true, "dark": true}

// UpdatePreferences saves theme + accent color on the account, so they
// follow the user across devices instead of living only in one browser's
// localStorage.
func (s *Service) UpdatePreferences(ctx context.Context, userID uuid.UUID, theme, gradientColor string) (repository.User, error) {
	if !validThemes[theme] {
		return repository.User{}, fmt.Errorf("invalid theme %q", theme)
	}
	if matched, _ := regexp.MatchString(`^#[0-9A-Fa-f]{6}$`, gradientColor); !matched {
		return repository.User{}, fmt.Errorf("gradientColor must be a #RRGGBB hex value")
	}
	return s.users.UpdatePreferences(ctx, userID, theme, gradientColor)
}

var validCurrencies = map[string]bool{"UAH": true, "USD": true, "EUR": true, "GBP": true, "PLN": true}

// UpdateBaseCurrency changes which currency the web/iOS clients convert
// amounts into for display.
func (s *Service) UpdateBaseCurrency(ctx context.Context, userID uuid.UUID, currency string) (repository.User, error) {
	if !validCurrencies[currency] {
		return repository.User{}, fmt.Errorf("unsupported currency %q", currency)
	}
	return s.users.UpdateBaseCurrency(ctx, userID, currency)
}

const maxGoalsLength = 4000

// UpdateGoals saves the free-form financial goals/plans text the user
// writes for themselves on the Profile page.
func (s *Service) UpdateGoals(ctx context.Context, userID uuid.UUID, goals string) (repository.User, error) {
	if len(goals) > maxGoalsLength {
		return repository.User{}, fmt.Errorf("goals text is too long (max %d characters)", maxGoalsLength)
	}
	return s.users.UpdateGoals(ctx, userID, goals)
}

const (
	maxNameLength = 80
	// A data: URI, base64-encoded. 300k chars is ~225KB of actual image
	// data — the frontend resizes to a small square before ever getting
	// here, so a legitimate avatar is nowhere near this; it's just a
	// backstop against someone posting something huge to a TEXT column.
	maxAvatarLength = 300_000
)

// UpdateProfile saves the display name + avatar shown on the profile
// page. avatar must be a data: URI (or "" to clear it) — never a remote
// URL, since nothing here fetches or validates a URL's content.
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, name, avatar string) (repository.User, error) {
	if len(name) > maxNameLength {
		return repository.User{}, fmt.Errorf("name is too long (max %d characters)", maxNameLength)
	}
	if len(avatar) > maxAvatarLength {
		return repository.User{}, fmt.Errorf("avatar image is too large")
	}
	if avatar != "" && !strings.HasPrefix(avatar, "data:image/") {
		return repository.User{}, fmt.Errorf("avatar must be an image data URI")
	}
	return s.users.UpdateProfile(ctx, userID, strings.TrimSpace(name), avatar)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *Service) findOrCreateAndIssueToken(ctx context.Context, sub, email string) (SignInResult, error) {
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
