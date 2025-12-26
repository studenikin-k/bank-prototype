package services

import (
	"context"
	"errors"
	"fmt"

	"bank-prototype/internal/models"
	"bank-prototype/internal/repository"
	"bank-prototype/internal/utils"
)

var (
	ErrUnauthorizedAccess   = errors.New("нет доступа к данному счёту")
	ErrAccountAlreadyClosed = errors.New("счёт уже закрыт")
	ErrAccountLimitReached  = errors.New("достигнут лимит активных счетов (максимум 5)")
)

const MaxActiveAccounts = 5

type AccountService struct {
	accountRepo *repository.AccountRepository
}

func NewAccountService(accountRepo *repository.AccountRepository) *AccountService {
	return &AccountService{
		accountRepo: accountRepo,
	}
}

func (s *AccountService) CreateAccount(ctx context.Context, userID string) (*models.Account, error) {
	utils.LogInfo("AccountService", fmt.Sprintf("Создание нового счёта для пользователя %s", userID))

	activeCount, err := s.accountRepo.CountActiveAccountsByUserID(ctx, userID)
	if err != nil {
		utils.LogError("AccountService", "Ошибка проверки лимита счетов", err)
		return nil, err
	}

	if activeCount >= MaxActiveAccounts {
		utils.LogWarning("AccountService", fmt.Sprintf("Пользователь %s достиг лимита активных счетов (%d/%d)", userID, activeCount, MaxActiveAccounts))
		return nil, ErrAccountLimitReached
	}

	account, err := s.accountRepo.Create(ctx, userID)
	if err != nil {
		utils.LogError("AccountService", fmt.Sprintf("Ошибка создания счёта для пользователя %s", userID), err)
		return nil, err
	}

	utils.LogSuccess("AccountService", fmt.Sprintf("Счёт %s успешно создан для пользователя %s (баланс: %.2f, активных счетов: %d/%d)", account.ID, userID, account.Balance, activeCount+1, MaxActiveAccounts))

	return account, nil
}

func (s *AccountService) GetUserAccounts(ctx context.Context, userID string) ([]models.Account, error) {
	utils.LogInfo("AccountService", fmt.Sprintf("Получение списка счетов пользователя %s", userID))

	accounts, err := s.accountRepo.GetByUserID(ctx, userID)
	if err != nil {
		utils.LogError("AccountService", fmt.Sprintf("Ошибка получения счетов пользователя %s", userID), err)
		return nil, err
	}

	activeCount := 0
	closedCount := 0
	for _, acc := range accounts {
		if acc.Status == "active" {
			activeCount++
		} else {
			closedCount++
		}
	}

	utils.LogSuccess("AccountService", fmt.Sprintf("Найдено счетов для пользователя %s: всего %d (активных: %d/%d, закрытых: %d)", userID, len(accounts), activeCount, MaxActiveAccounts, closedCount))

	return accounts, nil
}

func (s *AccountService) GetAccount(ctx context.Context, accountID, userID string) (*models.Account, error) {
	utils.LogInfo("AccountService", fmt.Sprintf("Получение информации о счёте %s", accountID))

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		utils.LogError("AccountService", fmt.Sprintf("Счёт %s не найден", accountID), err)
		return nil, repository.ErrAccountNotFound
	}

	if account.UserID != userID {
		utils.LogWarning("AccountService", fmt.Sprintf("Попытка доступа к чужому счёту %s пользователем %s", accountID, userID))
		return nil, ErrUnauthorizedAccess
	}

	if account.Status != "active" {
		utils.LogWarning("AccountService", fmt.Sprintf("Попытка доступа к закрытому счёту %s", accountID))
		return nil, repository.ErrAccountClosed
	}

	utils.LogSuccess("AccountService", fmt.Sprintf("Информация о счёте %s получена (баланс: %.2f)", accountID, account.Balance))

	return account, nil
}

func (s *AccountService) DeleteAccount(ctx context.Context, accountID, userID string) error {
	utils.LogInfo("AccountService", fmt.Sprintf("Закрытие счёта %s пользователем %s", accountID, userID))

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		utils.LogError("AccountService", fmt.Sprintf("Счёт %s не найден", accountID), err)
		return repository.ErrAccountNotFound
	}

	if account.UserID != userID {
		utils.LogWarning("AccountService", fmt.Sprintf("Попытка закрыть чужой счёт %s пользователем %s", accountID, userID))
		return ErrUnauthorizedAccess
	}

	if account.Status == "closed" {
		utils.LogWarning("AccountService", fmt.Sprintf("Счёт %s уже закрыт", accountID))
		return ErrAccountAlreadyClosed
	}

	if account.Balance > 0 {
		utils.LogInfo("AccountService", fmt.Sprintf("Перевод баланса %.2f со счёта %s на системный счёт", account.Balance, accountID))

		systemBalance, err := s.accountRepo.GetBalance(ctx, repository.SystemBankAccountID)
		if err != nil {
			utils.LogError("AccountService", "Ошибка получения баланса системного счёта", err)
			return fmt.Errorf("ошибка доступа к системному счёту: %w", err)
		}

		err = s.accountRepo.UpdateBalance(ctx, repository.SystemBankAccountID, systemBalance+account.Balance)
		if err != nil {
			utils.LogError("AccountService", "Ошибка перевода средств на системный счёт", err)
			return fmt.Errorf("ошибка перевода средств: %w", err)
		}

		err = s.accountRepo.UpdateBalance(ctx, accountID, 0)
		if err != nil {
			utils.LogError("AccountService", fmt.Sprintf("Ошибка обнуления баланса счёта %s", accountID), err)
			return fmt.Errorf("ошибка обнуления баланса: %w", err)
		}

		utils.LogSuccess("AccountService", fmt.Sprintf("Баланс %.2f успешно переведён на системный счёт", account.Balance))
	}

	err = s.accountRepo.UpdateStatus(ctx, accountID, "closed")
	if err != nil {
		utils.LogError("AccountService", fmt.Sprintf("Ошибка изменения статуса счёта %s", accountID), err)
		return err
	}

	utils.LogSuccess("AccountService", fmt.Sprintf("Счёт %s успешно закрыт", accountID))

	return nil
}

func (s *AccountService) VerifyOwnership(ctx context.Context, accountID, userID string) error {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return repository.ErrAccountNotFound
	}

	if account.UserID != userID {
		return ErrUnauthorizedAccess
	}

	if account.Status != "active" {
		return repository.ErrAccountClosed
	}

	return nil
}
