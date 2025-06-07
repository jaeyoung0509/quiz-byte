# 퀴즈 앱 백엔드 서버 개발 (instruction.md)

## 1. 프로젝트 개요
모바일 퀴즈 애플리케이션을 위한 백엔드 서버를 개발합니다. 사용자는 다양한 기술 분야의 퀴즈를 풀 수 있으며, 퀴즈 데이터는 외부 LLM을 통해 주기적으로 생성 및 업데이트됩니다.

### 현재 구현 상태 (2025)
- ✅ Clean Architecture 및 Domain-Driven Design 완전 적용
- ✅ Oracle Database 호환성 개선 (NULL 처리 강화)
- ✅ 트랜잭션 관리 계층 통합 구현
- ✅ AI 기반 답변 평가 시스템 완성
- ✅ 임베딩 기반 유사성 캐싱 시스템
- ✅ Google OAuth 2.0 인증 시스템
- ✅ 종합적인 테스트 커버리지 확보

## 2. 주요 기술 스택
- **언어**: Go 1.24+ (또는 최신 안정 버전)
- **웹 프레임워크 / API 핸들러**: Fiber v3
- **DB Access**: Sqlx (Oracle DB 연동 via godror driver)
- **데이터베이스**: Oracle Database (OCI Autonomous Database "상시 무료" 티어 활용 권장)
- **캐시**: Redis (임베딩 기반 유사성 캐싱 포함)
- **LLM 연동**: 
  - 퀴즈 답변/생성 보조: LangchainGo
  - 답변 생성 모델: Gemini API
  - 임베딩 서비스: OpenAI/Ollama
- **인증**: Google OAuth 2.0
- **로깅**: Zap Logger
- **스키마 관리**: 데이터베이스 마이그레이션 도구를 사용한 버전 관리 (golang-migrate/migrate)
- **트랜잭션 관리**: 통합 트랜잭션 매니저를 통한 일관된 데이터 무결성 보장

3. 배포 환경
클라우드 플랫폼: Oracle Cloud Infrastructure (OCI)

주요 활용 서비스 (OCI "상시 무료" Tier 권장):

컴퓨팅: Ampere A1 Compute VM (애플리케이션 서버 실행)

데이터베이스: Autonomous Database (퀴즈 데이터 저장)

네트워킹: Load Balancer (트래픽 분산 및 SSL/TLS 종료), VCN

모니터링 & 로깅: OCI Logging, OCI Monitoring, OCI APM (부분적)

## 4. 주요 기능 및 요구사항

### 4.1. 퀴즈 데이터 관리
- ✅ Fiber를 사용한 퀴즈 CRUD (Create, Read, Update, Delete) API 개발 완료
- ✅ Sqlx를 사용하여 Oracle DB에 퀴즈 데이터 저장 및 관리 구현
- ✅ NULL 값 처리 강화를 통한 Oracle DB 호환성 개선
- ✅ 트랜잭션 관리를 통한 데이터 무결성 보장

주기적인 퀴즈 업데이트:
- ✅ 배치(Batch) 작업을 통해 외부 상용 LLM과 통신하여 분야별 신규 퀴즈 데이터 생성
- ✅ 생성된 퀴즈를 데이터베이스에 주기적으로 저장 및 업데이트하는 로직 구현

### 4.2. 퀴즈 스키마 정의 및 관리
- ✅ 데이터베이스 스키마는 마이그레이션 도구를 사용하여 체계적으로 버전 관리
- ✅ Oracle Database의 sql.NullString을 활용한 NULL 처리 강화

필수 스키마 필드 정의 (현재 구현됨):

id (PK, NUMBER, 자동 증가 또는 UUID)

main_category (VARCHAR2, 예: "language", "network", "os", "system_design", "database") - 대분류

sub_category (VARCHAR2, 예: "python", "go", "tcp_ip", "linux", "microservices") - 중분류

question_text (CLOB 또는 VARCHAR2(4000)) - 질문 내용

answer_options (JSON 또는 CLOB, 객관식 보기 등) - 선택적, 문제 유형에 따라

correct_answer (CLOB 또는 VARCHAR2(4000)) - 정답

explanation (CLOB 또는 VARCHAR2(4000)) - 정답 해설

difficulty (NUMBER 또는 VARCHAR2, 예: 1-5점 또는 "상", "중", "하") - 난이도

correlation_coefficient (JSON 또는 별도 테이블) - 다른 질문과의 상관계수 (구체적 구조 설계 필요: related_question_id, coefficient_value, type 등)

created_at (TIMESTAMP WITH TIME ZONE)

updated_at (TIMESTAMP WITH TIME ZONE)

(선택) source (VARCHAR2) - 출제자/출처

(선택) tags (JSON 또는 VARCHAR2, 콤마로 구분된 태그)

### 4.3. LLM 연동 (퀴즈 답변 보조)
- ✅ LangchainGo 라이브러리를 활용한 AI 연동 구현
- ✅ Google Gemini API를 통한 퀴즈 생성 및 답변 평가
- ✅ OpenAI/Ollama 임베딩을 활용한 유사성 기반 답변 캐싱
- ✅ 다중 평가 지표를 통한 종합적 답변 분석:
  - 완성도(Completeness)
  - 관련성(Relevance) 
  - 정확성(Accuracy)
  - 키워드 매칭

참고: 주된 퀴즈 풀이 로직은 DB에 저장된 정답/해설을 활용하고, AI는 퀴즈 내용 이해, 유사 질문 생성, 키워드 추출 등에 활용됩니다.

### 4.4. 트랜잭션 관리 (새로 추가됨)
- ✅ 모든 서비스 계층에 일관된 트랜잭션 관리 구현
- ✅ TransactionManager 인터페이스를 통한 추상화
- ✅ 사용자 생성/업데이트, 퀴즈 시도 기록 등 중요 작업에 트랜잭션 경계 설정
- ✅ 데이터 무결성 보장 및 롤백 처리

### 4.5. 로깅 및 모니터링
Zap Logger를 사용하여 구조화된 로그(JSON 형식 권장)를 기록합니다.

애플리케이션 로그는 파일로 출력하고, OCI VM의 Unified Monitoring Agent를 통해 OCI Logging 서비스로 전송합니다.

슬로우 쿼리 로깅/모니터링:

애플리케이션 레벨: Sqlx 사용 부분에서 필요한 경우 쿼리 실행 시간을 로깅 (Zap 로거와 연동).

DB 레벨: OCI Autonomous Database의 Performance Hub 및 관련 OCI Monitoring 지표를 활용하여 DB 단의 슬로우 쿼리 분석 및 알림 설정.

### 4.6. 아키텍처 및 코드 품질
- ✅ Clean Architecture 원칙 완전 적용:
  - Presentation Layer: API Endpoints (Fiber 라우터 및 핸들러)
  - Business Logic Layer: 서비스 (애플리케이션 핵심 로직)
  - Data Access Layer: 리포지토리 (Sqlx를 사용한 DB 연동)
- ✅ 인터페이스(Interface) 기반 설계 및 의존성 주입(DI) 패턴 적용
- ✅ 포트 & 어댑터 패턴을 통한 외부 서비스 추상화
- ✅ 도메인 주도 설계(DDD) 원칙 준수
- ✅ 단위 테스트(Unit Test) 및 통합 테스트(Integration Test) 구현
- ✅ 종합적인 테스트 커버리지 확보

### 4.7. API 보안 및 인증 (구현 완료)
- ✅ Google OAuth 2.0 기반 사용자 인증 시스템
- ✅ JWT 토큰 기반 인가 시스템
- ✅ 미들웨어를 통한 API 보안 강화
- ✅ 구조화된 에러 처리 및 응답 시스템
OCI 로드밸런서에서 SSL/TLS 종료 및 공인 인증서 관리.

모바일 앱 클라이언트 인증 및 인가 방안 수립 및 적용 (예: API 키 검증, JWT 기반 사용자 인증 등).

## 5. 추가 고려 사항
- ✅ 컨테이너화: Docker를 사용하여 애플리케이션을 컨테이너화 완료
- ✅ 자원 효율성: Redis 캐싱을 통한 효율적 자원 사용 구현
- ✅ 환경변수 관리: 민감 정보의 환경변수 처리 구현
- ✅ 성능 최적화: 임베딩 기반 캐싱으로 LLM 응답 시간 개선

## 6. 현재 완료된 핵심 기능들
- **사용자 관리**: Google OAuth 2.0 인증, 사용자 프로필 관리
- **퀴즈 시스템**: 카테고리별 퀴즈 제공, AI 기반 답변 평가
- **캐싱 시스템**: Redis + 임베딩 기반 유사성 캐싱
- **트랜잭션 관리**: 모든 서비스 계층의 일관된 트랜잭션 처리
- **테스트 시스템**: 포괄적인 단위/통합 테스트 커버리지
- **API 문서화**: Swagger 기반 API 문서 자동 생성
- **배치 처리**: 대량 퀴즈 생성 및 데이터 시딩
- **추천 시스템**: 사용자 성과 기반 개인화 퀴즈 추천