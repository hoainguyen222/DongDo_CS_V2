# 🤝 CONTRIBUTING

> Cảm ơn bạn đã quan tâm đến việc đóng góp cho **Đông Đô CS V2**! 🎉

## Mục lục

- [Code of Conduct](#code-of-conduct)
- [Bắt đầu nhanh](#bắt-đầu-nhanh)
- [Workflow](#workflow)
- [Code style](#code-style)
- [Commit messages](#commit-messages)
- [Pull request process](#pull-request-process)
- [Code review checklist](#code-review-checklist)
- [Reporting bugs](#reporting-bugs)
- [Suggesting features](#suggesting-features)

---

## Code of Conduct

Dự án này tuân theo [Contributor Covenant](https://www.contributor-covenant.org/). Tham gia tích cực, tôn trọng, và xây dựng.

---

## Bắt đầu nhanh

### 1. Fork & clone

```bash
git clone https://github.com/your-username/DongDo_CS_V2.git
cd DongDo_CS_V2
git remote add upstream https://github.com/original/DongDo_CS_V2.git
```

### 2. Setup local dev

```bash
# Backend
go mod download
cp .env.example .env

# Frontend
cd web && pnpm install && cd ..

# Docker stack
docker compose up -d postgres redis qdrant
```

### 3. Tạo branch

```bash
git checkout -b feat/your-feature-name
# hoặc
git checkout -b fix/bug-description
```

### 4. Code, test, commit, push

```bash
# Make changes
make sqlc-gen     # nếu thay đổi SQL queries
go build ./...    # verify build
go test ./...
cd web && pnpm lint && pnpm type-check && pnpm build

# Commit
git add .
git commit -m "feat: add voice call recording endpoint"
git push origin feat/your-feature-name
```

### 5. Mở Pull Request

Vào GitHub → New Pull Request → chọn branch.

---

## Workflow

```text
main (production-ready)
  │
  ├── develop (integration)
  │     │
  │     ├── feat/xxx
  │     ├── fix/xxx
  │     └── docs/xxx
  │
  └── release/v2.1.0
```

- **`main`** — luôn deploy-ready, mỗi commit có tag
- **`develop`** — integration branch cho features
- **`feat/*`, `fix/*`, `docs/*`** — feature branches

---

## Code style

### Go

```bash
# Format (MUST run before commit)
gofmt -w .

# Lint
golangci-lint run

# Vet
go vet ./...
```

**Quy tắc:**

- Tên file: `snake_case.go` (vd: `user_repo.go`)
- Tên package: lowercase, no underscore
- Interface: kết thúc bằng `-er` (vd: `UserRepository`, `EventBus`)
- Struct: PascalCase, public fields documented
- Errors: wrap với `fmt.Errorf("...: %w", err)`
- Logs: dùng `zerolog` (đã setup), KHÔNG dùng `fmt.Println`

**Ví dụ:**

```go
// ✅ Good
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
    row, err := r.db.Auth.GetUserByEmail(ctx, authdb.GetUserByEmailParams{
        Username: strings.ToLower(email),
    })
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, nil  // not found is OK
        }
        return nil, fmt.Errorf("failed to get user by email: %w", err)
    }
    return toDomainUser(row), nil
}

// ❌ Bad
func (r *UserRepo) GetUser(e string) (*User, error) {
    row, _ := r.db.Auth.GetUserByEmail(context.Background(), authdb.GetUserByEmailParams{Username: e})
    return &User{Username: row.Username}, nil
}
```

### TypeScript / React

```bash
cd web
pnpm lint
pnpm type-check
pnpm format
```

**Quy tắc:**

- Dùng functional components + hooks
- Props: `interface FooProps { ... }` (không dùng `type`)
- Imports: `@/` alias cho internal, relative cho cùng module
- Export named exports (không default), trừ Next.js page components
- File naming: `PascalCase.tsx` cho components, `camelCase.ts` cho utilities

**Ví dụ:**

```tsx
// ✅ Good
import { useState, useEffect } from 'react';
import { Button } from '@/components/ui/Button';
import { api } from '@/lib/api';

interface InboxProps {
  sessionId: string;
  onSelect?: (caseId: string) => void;
}

export function Inbox({ sessionId, onSelect }: InboxProps) {
  const [cases, setCases] = useState<Case[]>([]);

  useEffect(() => {
    api.listCases({ sessionId }).then(setCases);
  }, [sessionId]);

  return <div>{cases.map(c => <CaseRow key={c.id} case={c} />)}</div>;
}

// ❌ Bad
export default function inbox(props) {
  const [x, setX] = useState([]);
  return <div onClick={() => props.onSelect && props.onSelect(c.id)}>{x.map(...)}</div>;
}
```

### SQL

```sql
-- ✅ Good: explicit columns, type-safe
-- name: GetUserByEmail :one
SELECT id, username, password_hash, salt, full_name, role, is_active, created_at
FROM users
WHERE LOWER(username) = LOWER($1);

-- ❌ Bad: SELECT *
SELECT * FROM users WHERE username = $1;
```

### SCSS

- BEM naming convention
- Không nested quá 3 levels
- Variables ở `web/src/styles/tokens/_variables.scss`

---

## Commit messages

Tuân theo [Conventional Commits](https://www.conventionalcommits.org/).

### Format

```
<type>(<scope>): <short description>

<longer description if needed>

<footer>
```

### Types

| Type | Mô tả | Ví dụ |
|---|---|---|
| `feat` | New feature | `feat(voice): add Asterisk AMI client` |
| `fix` | Bug fix | `fix(ws): prevent hub dropping clients on full buffer` |
| `docs` | Documentation only | `docs: add deployment guide` |
| `style` | Formatting (no code change) | `style: run gofmt` |
| `refactor` | Code change (no feature/fix) | `refactor(usecase): extract common session logic` |
| `test` | Add/fix tests | `test(voice): add unit test for EndCall` |
| `chore` | Build/CI/tooling | `chore(deps): bump gin to v1.10.0` |
| `perf` | Performance | `perf(db): batch insert messages` |

### Scope

`voice`, `chat`, `auth`, `admin`, `frontend`, `infra`, `docker`, `docs`, ...

### Ví dụ

```text
feat(voice): add /api/voice/accept endpoint

- Backend now accepts call via AMI Action: Originate
- Bridge agent SIP endpoint into active call
- Broadcast WS event call_status when active
- Add smoke test step

Closes #123
```

---

## Pull request process

### PR template

```markdown
## What
<!-- Short description of changes -->

## Why
<!-- Motivation / context -->

## How
<!-- Technical approach, breaking changes -->

## Testing
<!-- How was it tested? -->
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing done

## Screenshots
<!-- If UI changes, attach screenshots -->

## Checklist
- [ ] Code follows style guide
- [ ] Self-reviewed
- [ ] Comments added for complex logic
- [ ] Documentation updated
- [ ] No new warnings
- [ ] sqlc-gen run (if SQL changed)
- [ ] Migrations tested up AND down
```

### Process

1. **Open PR** với title theo Conventional Commits format
2. **CI passes** — backend build, lint, test; frontend lint, type-check, build
3. **Review** — ít nhất 1 approval từ maintainer
4. **Squash & merge** khi approved
5. **Delete branch** sau khi merge

### Review SLA

- Initial review: trong vòng **2 ngày làm việc**
- Bug fixes: ưu tiên cao
- Features: best effort

---

## Code review checklist

### Author (trước khi mở PR)

- [ ] Tests pass locally
- [ ] Code formatted (`gofmt`, `prettier`)
- [ ] No new lint warnings
- [ ] sqlc regenerated nếu có SQL changes
- [ ] Migrations up AND down tested
- [ ] Documentation updated (README, docs/, OpenAPI)
- [ ] PR description đầy đủ
- [ ] Linked issue (nếu có)

### Reviewer

- [ ] **Correctness** — Logic đúng? Edge cases?
- [ ] **Security** — Auth/authz, input validation, SQL injection, XSS?
- [ ] **Performance** — N+1 queries, memory leaks, blocking calls?
- [ ] **Reliability** — Error handling, graceful shutdown, retries?
- [ ] **Style** — Follow conventions?
- [ ] **Tests** — Adequate coverage? Edge cases?
- [ ] **Docs** — Public APIs documented? Migration guide updated?
- [ ] **Backwards compat** — Breaking changes được document?
- [ ] **RBAC** — Permissions check đúng?

---

## Reporting bugs

Mở issue với template:

```markdown
## Bug description
<!-- What happened? -->

## Steps to reproduce
1. ...
2. ...
3. ...

## Expected behavior
<!-- What should happen? -->

## Actual behavior
<!-- What actually happens? -->

## Environment
- OS: macOS 14 / Ubuntu 22.04 / ...
- Go version: 1.21
- Node version: 20
- Docker version: 24
- Asterisk version: 20-current

## Logs
```
<paste relevant logs>
```

## Screenshots
<!-- If applicable -->
```

---

## Suggesting features

Mở issue với template:

```markdown
## Feature description
<!-- What do you want? -->

## Motivation
<!-- Why is this useful? -->

## Alternatives considered
<!-- Other approaches? -->

## Implementation sketch
<!-- Optional: how would you build this? -->

## Acceptance criteria
- [ ] ...
- [ ] ...
```

---

## Cấu trúc thư mục (tóm tắt)

Xem [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) và [README.md](./README.md) §Cấu trúc thư mục.

### Khi thêm file mới

| Loại | Vị trí |
|---|---|
| SQL query | `db/queries/<domain>/<name>.sql` rồi `make sqlc-gen` |
| Migration | `db/migrations/<seq>_<name>.sql` + copy vào `internal/repository/postgres/migrations/` |
| Use case | `internal/usecase/<name>_usecase.go` |
| Handler | `internal/delivery/http/<name>_handlers.go` |
| Worker | `internal/worker/<name>_worker.go` |
| API endpoint | Update `internal/delivery/http/router.go` + `docs/API.md` |
| Frontend page | `web/src/app/<route>/page.tsx` |
| Frontend component | `web/src/components/<area>/<Name>.tsx` |
| Doc | `docs/<TOPIC>.md` + add link in README |

---

## Community

- GitHub Issues: bug reports, feature requests
- GitHub Discussions: Q&A, ideas
- Slack: `#dongdo-dev` (internal)

---

## License

Bằng việc đóng góp, bạn đồng ý license contributions theo [MIT](./LICENSE).
