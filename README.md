# Quiz Byte Backend

## 소개
Quiz Byte는 컴퓨터 공학 및 IT 분야의 다양한 퀴즈를 제공하는 백엔드 시스템입니다. Go 언어, Oracle DB, GORM, Fiber 프레임워크를 기반으로 설계되었습니다.

## 주요 기능
- 카테고리/하위카테고리 기반 퀴즈 제공
- 객관식/서술형 퀴즈 지원
- LLM(대형 언어 모델) 기반 자동 채점
- RESTful API 제공
- 통합 테스트 및 DB 마이그레이션 지원

## 프로젝트 구조
```
cmd/                # 실행 엔트리포인트(main.go 등)
internal/           # 핵심 도메인, 서비스, 핸들러, DB, 로거 등
  domain/           # 도메인 모델
  repository/       # DB 모델 및 쿼리
  handler/          # HTTP 핸들러
  service/          # 비즈니스 로직
  logger/           # 로깅
  database/         # DB 연결/마이그레이션
  dto/              # API DTO
configs/            # 설정 파일
config/             # 환경설정
pkg/                # 외부 패키지

tests/integration/  # 통합 테스트 및 샘플 데이터
```

## 개발 환경
- Go 1.20 이상
- Oracle DB (로컬/원격)
- Docker, Docker Compose (테스트/로컬 개발)
- (선택) Oracle Instant Client

## 실행 방법
1. 의존성 설치
```bash
go mod tidy
```
2. 환경 변수 또는 config/config.yaml 설정
3. DB 마이그레이션
```bash
go run cmd/migrate/main.go
```
4. 서버 실행
```bash
go run cmd/api/main.go
```
5. 통합 테스트
```bash
go test ./tests/integration
```

## 환경 변수 예시
```
DB_USER=system
DB_PASSWORD=oracle
DB_HOST=localhost
DB_PORT=1521
DB_SERVICE_NAME=FREE
```

## 기여 및 문의
- PR/이슈 환영
- 문의: jaeyeong.i.dev@gmail.com