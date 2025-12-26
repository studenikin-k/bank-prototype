-- Включение расширения для генерации UUID
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
                       id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                       name TEXT UNIQUE NOT NULL,
                       password_hash TEXT NOT NULL,
                       created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_users_name ON users(name);


CREATE TABLE accounts (
                          id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                          user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                          balance DECIMAL(15,2) NOT NULL DEFAULT 100.00,
                          status TEXT NOT NULL DEFAULT 'active',
                          created_at TIMESTAMPTZ DEFAULT NOW()
);


CREATE TABLE transactions (
                              id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                              type TEXT NOT NULL CHECK (type IN ('transfer', 'payment')),
                              from_account_id UUID NOT NULL REFERENCES accounts(id),
                              to_account_id UUID NOT NULL REFERENCES accounts(id),
                              amount DECIMAL(15,2) NOT NULL,
                              fee_percent SMALLINT NOT NULL CHECK (fee_percent IN (1,3)),
                              fee_amount DECIMAL(15,2) NOT NULL,
                              total_debit DECIMAL(15,2) NOT NULL,
                              fee_account_id UUID NOT NULL REFERENCES accounts(id),
                              status TEXT NOT NULL DEFAULT 'pending',
                              created_at TIMESTAMPTZ DEFAULT NOW()
);


CREATE INDEX idx_tx_from ON transactions(from_account_id);
CREATE INDEX idx_tx_to ON transactions(to_account_id);
CREATE INDEX idx_tx_created ON transactions(created_at);


-- Системный пользователь для хранения комиссий
INSERT INTO users (id, name, password_hash)
VALUES ('00000000-0000-0000-0000-000000000000', 'SYSTEM', 'none');

-- Системный счёт банка для сбора комиссий
INSERT INTO accounts (id, user_id, balance, status)
VALUES ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000000', 0.00, 'active');
