# Agent Connector Makefile

.PHONY: all test test-verbose test-coverage test-race test-short build clean lint fmt vet

# 默认目标
all: test

# 运行所有测试
test:
	go test -v ./...

# 运行简短测试（跳过集成测试）
test-short:
	go test -v -short ./...

# 运行详细测试
test-verbose:
	go test -v -race ./...

# 运行测试并生成覆盖率报告
test-coverage:
	go test -v -cover ./...
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 运行竞态检测测试
test-race:
	go test -v -race ./...

# 运行基准测试
benchmark:
	go test -bench=. -benchmem ./...

# 构建示例程序
build:
	go build -o bin/simple_test ./examples/simple_test/

# 运行示例程序
run-example: build
	./bin/simple_test -duration 30s

# 格式化代码
fmt:
	go fmt ./...

# 运行 go vet
vet:
	go vet ./...

# 运行 linter (需要安装 golangci-lint)
lint:
	golangci-lint run ./...

# 清理生成的文件
clean:
	rm -f coverage.out coverage.html
	rm -rf bin/

# 下载依赖
deps:
	go mod download
	go mod tidy

# 更新依赖
update-deps:
	go get -u ./...
	go mod tidy

# 运行特定包的测试
test-protocol:
	go test -v ./protocol/...

test-websocket:
	go test -v ./websocket/...

test-mqtt:
	go test -v ./mqtt/...

test-router:
	go test -v ./router/...

# 运行集成测试（需要外部服务）
test-integration:
	go test -v -run Integration ./...

# 帮助信息
help:
	@echo "Available targets:"
	@echo "  make test           - Run all tests"
	@echo "  make test-short     - Run short tests (skip integration tests)"
	@echo "  make test-verbose   - Run tests with verbose output and race detection"
	@echo "  make test-coverage  - Run tests and generate coverage report"
	@echo "  make test-race      - Run tests with race detector"
	@echo "  make benchmark      - Run benchmark tests"
	@echo "  make build          - Build example program"
	@echo "  make run-example    - Run example program"
	@echo "  make fmt            - Format code"
	@echo "  make vet            - Run go vet"
	@echo "  make lint           - Run linter"
	@echo "  make clean          - Clean generated files"
	@echo "  make deps           - Download dependencies"
	@echo "  make update-deps    - Update dependencies"
	@echo "  make test-protocol  - Test protocol package"
	@echo "  make test-websocket - Test websocket package"
	@echo "  make test-mqtt      - Test mqtt package"
	@echo "  make test-router    - Test router package"
	@echo "  make test-integration - Run integration tests"
