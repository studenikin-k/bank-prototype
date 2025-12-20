package services

import (
	"bank-prototype/internal/utils"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	jwtSecret     string
	jwtExpiration time.Duration
}

func NewAuthService(secret string, expiration time.Duration) *AuthService {
	utils.LogSuccess("AuthService", fmt.Sprintf("Инициализирован сервис аутентификации (TTL: %v)", expiration))
	return &AuthService{
		jwtSecret:     secret,
		jwtExpiration: expiration,
	}
}

func (s *AuthService) HashPassword(password string) (string, error) {
	utils.LogDebug("AuthService", "Хеширование пароля...")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		utils.LogError("AuthService", "Ошибка хеширования пароля", err)
		return "", err
	}

	utils.LogSuccess("AuthService", "Пароль успешно захеширован")
	return string(hashedPassword), nil
}

func (s *AuthService) CheckPasswordHash(password, hash string) error {
	utils.LogDebug("AuthService", "Проверка пароля...")

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		utils.LogWarning("AuthService", "Неверный пароль")
		return err
	}

	utils.LogSuccess("AuthService", "Пароль верный")
	return nil
}

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func (s *AuthService) GenerateToken(userID string) (string, error) {
	utils.LogDebug("AuthService", fmt.Sprintf("Генерация JWT токена для пользователя: %s", userID))

	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtExpiration)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		utils.LogError("AuthService", "Ошибка подписи токена", err)
		return "", err
	}

	utils.LogSuccess("AuthService", fmt.Sprintf("JWT токен создан для пользователя: %s", userID))
	return signedToken, nil
}

func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	utils.LogDebug("AuthService", "Валидация JWT токена...")

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		utils.LogWarning("AuthService", "Невалидный токен")
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		utils.LogWarning("AuthService", "Токен не прошёл валидацию")
		return nil, errors.New("invalid token")
	}

	utils.LogSuccess("AuthService", fmt.Sprintf("Токен валиден для пользователя: %s", claims.UserID))
	return claims, nil
}
