#!/usr/bin/env bash
# Pre-Release Validation Script for PubSub-Go Library
# This script runs all quality checks before creating a release
# EXACTLY matches CI checks + additional validations

set -e  # Exit on first error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Header
echo ""
echo "========================================"
echo "  PubSub-Go - Pre-Release Check"
echo "========================================"
echo ""

# Track overall status
ERRORS=0
WARNINGS=0

# 1. Check Go version
log_info "Checking Go version..."
GO_VERSION=$(go version | awk '{print $3}')
REQUIRED_VERSION="go1.21"
if [[ "$GO_VERSION" < "$REQUIRED_VERSION" ]]; then
    log_error "Go version $REQUIRED_VERSION+ required, found $GO_VERSION"
    ERRORS=$((ERRORS + 1))
else
    log_success "Go version: $GO_VERSION"
fi
echo ""

# 2. Check git status
log_info "Checking git status..."
if git diff-index --quiet HEAD --; then
    log_success "Working directory is clean"
else
    log_warning "Uncommitted changes detected"
    git status --short
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 3. Code formatting check (EXACT CI command)
log_info "Checking code formatting (gofmt -l .)..."
UNFORMATTED=$(gofmt -l . 2>/dev/null | grep -v "^vendor/" || true)
if [ -n "$UNFORMATTED" ]; then
    log_error "The following files need formatting:"
    echo "$UNFORMATTED"
    echo ""
    log_info "Run: gofmt -w ."
    ERRORS=$((ERRORS + 1))
else
    log_success "All files are properly formatted"
fi
echo ""

# 4. Go vet
log_info "Running go vet..."
if go vet ./... 2>&1; then
    log_success "go vet passed"
else
    log_error "go vet failed"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 5. Build all packages
log_info "Building all packages..."
if go build ./... 2>&1; then
    log_success "Build successful"
else
    log_error "Build failed"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 6. go.mod validation
log_info "Validating go.mod..."
go mod verify
if [ $? -eq 0 ]; then
    log_success "go.mod verified"
else
    log_error "go.mod verification failed"
    ERRORS=$((ERRORS + 1))
fi

# Check if go.mod needs tidying
go mod tidy
if git diff --quiet go.mod go.sum 2>/dev/null; then
    log_success "go.mod is tidy"
else
    log_warning "go.mod needs tidying (run 'go mod tidy')"
    git diff go.mod go.sum 2>/dev/null || true
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 6.5. Check for replace directives in go.mod
log_info "Checking for replace directives in go.mod..."
REPLACE_COUNT=$(grep "^replace " go.mod | wc -l || echo "0")

if [ "$REPLACE_COUNT" -gt 0 ]; then
    log_error "Found $REPLACE_COUNT replace directive(s) in go.mod - NOT allowed for release!"
    echo ""
    echo "Replace directives found:"
    grep "^replace " go.mod | sed 's/^/  /'
    echo ""
    log_info "Replace directives are for local development only."
    log_info "Remove all replace directives before release:"
    echo "  1. Ensure all dependencies are published to their repositories"
    echo "  2. Remove replace directives from go.mod"
    echo "  3. Run: go mod tidy"
    echo "  4. Verify: go build ./..."
    echo ""
    ERRORS=$((ERRORS + 1))
else
    log_success "No replace directives found in go.mod"
fi
echo ""

# 6.6. Check module path consistency
log_info "Checking module path consistency..."
MODULE_PATH=$(grep "^module " go.mod | awk '{print $2}')
README_PATH=$(grep "go get github.com" README.md | head -1 | awk '{print $3}' | sed 's/@latest$//' || echo "NOT_FOUND")

if [ "$MODULE_PATH" = "$README_PATH" ]; then
    log_success "Module path consistent: $MODULE_PATH"
elif [ "$README_PATH" = "NOT_FOUND" ]; then
    log_warning "Could not verify module path in README.md"
    WARNINGS=$((WARNINGS + 1))
else
    log_error "Module path mismatch!"
    echo "  go.mod:    $MODULE_PATH"
    echo "  README.md: $README_PATH"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 7. Run tests
log_info "Running tests..."
TEST_OUTPUT=$(go test ./... 2>&1)

if echo "$TEST_OUTPUT" | grep -q "FAIL"; then
    log_error "Tests failed"
    echo "$TEST_OUTPUT"
    echo ""
    ERRORS=$((ERRORS + 1))
elif echo "$TEST_OUTPUT" | grep -q "PASS\|ok"; then
    log_success "All tests passed"
else
    log_error "Unexpected test output"
    echo "$TEST_OUTPUT"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 8. Test coverage check
log_info "Checking test coverage..."
OVERALL_COVERAGE=$(go test -cover ./... 2>&1 | grep "coverage:" | tail -1 | awk '{print $5}' | sed 's/%//' || echo "0")
MODEL_COVERAGE=$(go test -cover ./model 2>&1 | grep "coverage:" | awk '{print $5}' | sed 's/%//' || echo "0")
RETRY_COVERAGE=$(go test -cover ./retry 2>&1 | grep "coverage:" | awk '{print $5}' | sed 's/%//' || echo "0")

echo "  • Overall coverage: ${OVERALL_COVERAGE}%"
echo "  • model/ coverage:  ${MODEL_COVERAGE}%"
echo "  • retry/ coverage:  ${RETRY_COVERAGE}%"

COVERAGE_OK=1
if [ -n "$MODEL_COVERAGE" ] && awk -v cov="$MODEL_COVERAGE" 'BEGIN {exit !(cov >= 90.0)}'; then
    log_success "model/ coverage meets requirement (≥90%)"
else
    log_error "model/ coverage below 90% (${MODEL_COVERAGE}%)"
    ERRORS=$((ERRORS + 1))
    COVERAGE_OK=0
fi

if [ -n "$OVERALL_COVERAGE" ] && awk -v cov="$OVERALL_COVERAGE" 'BEGIN {exit !(cov >= 70.0)}'; then
    log_success "Overall coverage meets requirement (≥70%)"
else
    log_warning "Overall coverage below 70% (${OVERALL_COVERAGE}%) - aim for this before v1.0.0"
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 9. Check critical documentation files
log_info "Checking documentation..."
DOCS_MISSING=0
REQUIRED_DOCS="README.md LICENSE"

for doc in $REQUIRED_DOCS; do
    if [ ! -f "$doc" ]; then
        log_error "Missing: $doc"
        DOCS_MISSING=1
        ERRORS=$((ERRORS + 1))
    fi
done

if [ $DOCS_MISSING -eq 0 ]; then
    log_success "All critical documentation files present"
fi
echo ""

# 10. Check LICENSE file
log_info "Checking LICENSE..."
if [ -f "LICENSE" ]; then
    if grep -q "MIT License" LICENSE; then
        log_success "LICENSE file present (MIT)"
    else
        log_warning "LICENSE file exists but may not be MIT"
        WARNINGS=$((WARNINGS + 1))
    fi
else
    log_error "LICENSE file missing (required for Open Source)"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 11. Check for godoc comments
log_info "Checking godoc documentation..."
MISSING_DOCS=0

# Check for package-level doc
if [ ! -f "doc.go" ]; then
    log_warning "No doc.go found (package-level documentation)"
    MISSING_DOCS=1
fi

# Check exported types have docs (simple heuristic)
EXPORTED_WITHOUT_DOCS=$(grep -r "^type [A-Z]" --include="*.go" --exclude="*_test.go" . 2>/dev/null | wc -l || echo "0")
GODOC_COMMENTS=$(grep -r "^// [A-Z][a-zA-Z]* " --include="*.go" --exclude="*_test.go" . 2>/dev/null | wc -l || echo "0")

if [ "$EXPORTED_WITHOUT_DOCS" -gt "$GODOC_COMMENTS" ]; then
    log_warning "Some exported types may be missing godoc comments"
    log_info "Run: go doc -all | less  # to verify documentation"
    MISSING_DOCS=1
fi

if [ $MISSING_DOCS -eq 0 ]; then
    log_success "Godoc documentation appears complete"
else
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 12. Check migrations are embedded
log_info "Checking embedded migrations..."
MIGRATION_FILES=$(find migrations -name "*.sql" 2>/dev/null | wc -l || echo "0")
if [ "$MIGRATION_FILES" -ge 3 ]; then
    log_success "Found $MIGRATION_FILES migration files"

    # Check if migrations.go has embed directive
    if grep -q "//go:embed migrations/\*.sql" migrations.go 2>/dev/null; then
        log_success "Migrations properly embedded in migrations.go"
    else
        log_error "migrations.go missing //go:embed directive"
        ERRORS=$((ERRORS + 1))
    fi
else
    log_error "Expected at least 3 migration files, found $MIGRATION_FILES"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 13. Check examples compile
log_info "Checking examples..."
if [ -d "examples" ]; then
    EXAMPLE_DIRS=$(find examples -name "main.go" -exec dirname {} \; 2>/dev/null)
    EXAMPLES_OK=1

    for dir in $EXAMPLE_DIRS; do
        # Build in the directory context
        if (cd "$dir" && go build . 2>&1 > /dev/null); then
            log_success "Example builds: $dir"
        else
            log_error "Example failed to build: $dir"
            ERRORS=$((ERRORS + 1))
            EXAMPLES_OK=0
        fi
    done

    if [ $EXAMPLES_OK -eq 1 ]; then
        log_success "All examples compile"
    fi
else
    log_warning "No examples/ directory found"
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 14. Check for sensitive data (basic check)
log_info "Checking for sensitive data..."
SENSITIVE_FOUND=0

# Check for common secrets patterns
if grep -r "password.*=.*\"[^\"]\+\"" --include="*.go" --exclude-dir=vendor --exclude="*_test.go" . 2>/dev/null | grep -v "user:password@" | grep -q .; then
    log_error "Found hardcoded passwords in source code"
    SENSITIVE_FOUND=1
fi

if grep -r "token.*=.*\"[^\"]\+\"" --include="*.go" --exclude-dir=vendor --exclude="*_test.go" . 2>/dev/null | grep -q .; then
    log_warning "Found possible hardcoded tokens"
    SENSITIVE_FOUND=1
fi

if [ $SENSITIVE_FOUND -eq 0 ]; then
    log_success "No obvious sensitive data found"
else
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 15. Verify README badges and links
log_info "Checking README.md..."
README_CHECKS=0

if ! grep -q "go.dev/badge" README.md 2>/dev/null; then
    log_warning "README.md missing Go version badge"
    README_CHECKS=1
fi

if ! grep -q "License" README.md 2>/dev/null; then
    log_warning "README.md missing License badge/section"
    README_CHECKS=1
fi

# Check module path in README examples
if grep -q "github.com/coregx/pubsub" README.md; then
    log_success "README.md contains import examples"
else
    log_warning "README.md may be missing import examples"
    README_CHECKS=1
fi

if [ $README_CHECKS -eq 0 ]; then
    log_success "README.md looks good"
else
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# Summary
echo "========================================"
echo "  Summary"
echo "========================================"
echo ""

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    log_success "✅ All checks passed! Ready for release."
    echo ""
    log_info "Next steps for v1.0.0 release:"
    echo ""
    echo "  1. Finalize version and create tag:"
    echo "     git tag -a v1.0.0 -m \"Release v1.0.0: Production-ready PubSub library\""
    echo ""
    echo "  2. Push tag to GitHub:"
    echo "     git push origin v1.0.0"
    echo ""
    echo "  3. Create GitHub release:"
    echo "     - Go to GitHub repository"
    echo "     - Releases → Draft a new release"
    echo "     - Choose tag: v1.0.0"
    echo "     - Release title: v1.0.0 - Production Ready"
    echo "     - Add release notes (features, fixes, breaking changes)"
    echo ""
    echo "  4. Verify on pkg.go.dev:"
    echo "     - Visit: https://pkg.go.dev/github.com/[yourorg]/pubsub-go"
    echo "     - Wait ~15 minutes for indexing"
    echo ""
    echo "  5. Announce release:"
    echo "     - Twitter/X, LinkedIn, Reddit (r/golang)"
    echo "     - Blog post (optional)"
    echo ""
    exit 0
elif [ $ERRORS -eq 0 ]; then
    log_warning "⚠️  Checks completed with $WARNINGS warning(s)"
    echo ""
    log_info "Review warnings above. You can proceed with release if acceptable."
    echo ""
    exit 0
else
    log_error "❌ Checks failed with $ERRORS error(s) and $WARNINGS warning(s)"
    echo ""
    log_error "Fix errors before creating release!"
    echo ""
    echo "Common fixes:"
    echo "  • Run: gofmt -w ."
    echo "  • Run: go mod tidy"
    echo "  • Run: go test ./..."
    echo "  • Check: .claude/STATUS.md for open tasks"
    echo "  • Check: docs/dev/kanban/todo/ for critical tasks"
    echo ""
    exit 1
fi
