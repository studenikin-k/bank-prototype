-- Удаляем зависимые таблицы
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;

-- Пересоздаём таблицу счетов с TEXT ID
CREATE TABLE accounts (
                          id TEXT PRIMARY KEY,
                          user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                          balance DECIMAL(15,2) NOT NULL DEFAULT 100.00,
                          status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'closed')),
                          created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_accounts_user_id ON accounts(user_id);
CREATE INDEX idx_accounts_status ON accounts(status);


-- Пересоздаём таблицу транзакций с TEXT для account_id
CREATE TABLE transactions (
                              id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                              type TEXT NOT NULL CHECK (type IN ('transfer', 'payment')),
                              from_account_id TEXT NOT NULL REFERENCES accounts(id),
                              to_account_id TEXT NOT NULL REFERENCES accounts(id),
                              amount DECIMAL(15,2) NOT NULL,
                              fee_percent SMALLINT NOT NULL CHECK (fee_percent IN (1,3)),
                              fee_amount DECIMAL(15,2) NOT NULL,
                              total_debit DECIMAL(15,2) NOT NULL,
                              fee_account_id TEXT NOT NULL REFERENCES accounts(id),
                              status TEXT NOT NULL DEFAULT 'pending',
                              created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_tx_from ON transactions(from_account_id);
CREATE INDEX idx_tx_to ON transactions(to_account_id);
CREATE INDEX idx_tx_created ON transactions(created_at);


-- Системный счёт банка для сбора комиссий (14 цифр, начинается с нулей)
INSERT INTO accounts (id, user_id, balance, status)
VALUES ('00000000000001', '00000000-0000-0000-0000-000000000000', 0.00, 'active');

