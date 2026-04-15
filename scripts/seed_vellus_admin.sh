#!/usr/bin/env bash
# seed_vellus_admin.sh — Seed idempotente do usuário admin do tenant Vellus.
#
# Uso:
#   POSTGRES_DSN="postgres://user:pass@host/db" \
#   ADMIN_EMAIL="milton@vellus.tech" \
#   ADMIN_PASSWORD="..." \
#   ./scripts/seed_vellus_admin.sh
#
# O script:
#   1. Verifica que POSTGRES_DSN e ADMIN_PASSWORD estão definidos
#   2. Busca o tenant_id do slug 'vellus'
#   3. Gera hash Argon2id da senha via Go one-liner
#   4. INSERT com ON CONFLICT para o usuário admin
#   5. Confirma o registro criado
set -euo pipefail

# --- Validações ---
if [[ -z "${POSTGRES_DSN:-}" ]]; then
  echo "ERROR: POSTGRES_DSN não está definido." >&2
  echo "Exemplo: export POSTGRES_DSN=\"postgres://user:pass@localhost/argoclaw\"" >&2
  exit 1
fi

if [[ -z "${ADMIN_PASSWORD:-}" ]]; then
  echo "ERROR: ADMIN_PASSWORD não está definido." >&2
  echo "Exemplo: export ADMIN_PASSWORD=\"uma-senha-forte\"" >&2
  exit 1
fi

ADMIN_EMAIL="${ADMIN_EMAIL:-milton@vellus.tech}"
ADMIN_DISPLAY_NAME="${ADMIN_DISPLAY_NAME:-Milton Silva}"

echo "==> Buscando tenant_id do slug 'vellus'..."
TENANT_ID=$(psql "$POSTGRES_DSN" -t -A -c "SELECT id FROM tenants WHERE slug = 'vellus' LIMIT 1;")

if [[ -z "$TENANT_ID" ]]; then
  echo "ERROR: Tenant 'vellus' não encontrado no banco." >&2
  echo "Execute a migration 000035 primeiro: argoclaw migrate up" >&2
  exit 1
fi

echo "    tenant_id: $TENANT_ID"

# --- Hash Argon2id via Go one-liner ---
# Requer Go instalado. Usa os parâmetros OWASP recomendados:
#   time=2, memory=64MB, threads=1, keyLen=32
echo "==> Gerando hash Argon2id da senha..."

GO_BIN="${GO_BIN:-go}"
# Tenta detectar o caminho do Go no Windows (via WSL ou Git Bash)
if [[ "$OSTYPE" == "msys"* ]] || [[ "$OSTYPE" == "cygwin"* ]]; then
  GO_BIN="/c/Program Files/Go/bin/go.exe"
fi

if ! command -v "$GO_BIN" &>/dev/null && [[ "$GO_BIN" == "go" ]]; then
  echo "ERROR: Go não encontrado. Instale Go ou defina GO_BIN=/path/to/go" >&2
  exit 1
fi

PASSWORD_HASH=$("$GO_BIN" run - <<'EOF'
package main

import (
    "fmt"
    "os"
    "golang.org/x/crypto/argon2"
    "crypto/rand"
    "encoding/base64"
)

func main() {
    password := os.Getenv("ADMIN_PASSWORD")
    if password == "" {
        fmt.Fprintln(os.Stderr, "ADMIN_PASSWORD not set")
        os.Exit(1)
    }
    salt := make([]byte, 16)
    if _, err := rand.Read(salt); err != nil {
        fmt.Fprintln(os.Stderr, "failed to generate salt:", err)
        os.Exit(1)
    }
    // Parâmetros OWASP: time=2, memory=64MB, threads=1, keyLen=32
    hash := argon2.IDKey([]byte(password), salt, 2, 64*1024, 1, 32)
    saltB64 := base64.RawStdEncoding.EncodeToString(salt)
    hashB64 := base64.RawStdEncoding.EncodeToString(hash)
    fmt.Printf("$argon2id$v=19$m=65536,t=2,p=1$%s$%s", saltB64, hashB64)
}
EOF
)

if [[ -z "$PASSWORD_HASH" ]]; then
  echo "ERROR: Falha ao gerar hash Argon2id" >&2
  exit 1
fi

echo "    hash gerado com sucesso (argon2id)"

# --- Gerar UUID para o usuário ---
USER_ID=$(psql "$POSTGRES_DSN" -t -A -c "SELECT gen_random_uuid();")

# --- INSERT com ON CONFLICT ---
echo "==> Inserindo/atualizando usuário admin no tenant vellus..."
psql "$POSTGRES_DSN" <<SQL
INSERT INTO users (id, email, display_name, password_hash, status, created_at, updated_at)
VALUES (
    '${USER_ID}',
    lower('${ADMIN_EMAIL}'),
    '${ADMIN_DISPLAY_NAME}',
    '${PASSWORD_HASH}',
    'active',
    NOW(),
    NOW()
)
ON CONFLICT (email) DO UPDATE
    SET password_hash = EXCLUDED.password_hash,
        status = 'active',
        updated_at = NOW();
SQL

# Buscar o user_id real (pode ter vindo do ON CONFLICT)
REAL_USER_ID=$(psql "$POSTGRES_DSN" -t -A -c "SELECT id FROM users WHERE email = lower('${ADMIN_EMAIL}') LIMIT 1;")

psql "$POSTGRES_DSN" <<SQL
INSERT INTO tenant_users (tenant_id, user_id, role, joined_at)
VALUES (
    '${TENANT_ID}',
    '${REAL_USER_ID}',
    'admin',
    NOW()
)
ON CONFLICT (tenant_id, user_id) DO UPDATE
    SET role = 'admin';
SQL

# --- Confirmação ---
echo ""
echo "==> Verificando registro criado..."
psql "$POSTGRES_DSN" <<SQL
SELECT
    u.email,
    u.display_name,
    u.status,
    tu.role,
    t.slug AS tenant_slug,
    t.operator_level
FROM users u
JOIN tenant_users tu ON tu.user_id = u.id
JOIN tenants t ON t.id = tu.tenant_id
WHERE u.email = lower('${ADMIN_EMAIL}')
  AND t.slug = 'vellus';
SQL

echo ""
echo "==> Seed concluido com sucesso!"
echo "    Email:    $ADMIN_EMAIL"
echo "    Tenant:   vellus (operator_level=1)"
echo "    Role:     admin"
