# Xiaomi BLE 제품 추가

[English](ADDING-DEVICES.en.md)

브리지는 전송 파싱, 제품 데이터 해석 및 HomeKit 표현을 분리합니다. 새 제품을
추가할 때 MQTT 런타임에 PDID 조건문을 직접 늘리지 않습니다.

```text
MQTT envelope
  -> 제품 레지스트리(PDID, 모델, 유형)
  -> BLE 디코더(EID/edata -> 타입이 있는 Update)
  -> 액세서리 팩토리(유형 -> HomeKit 서비스)
  -> 안정적인 AID와 Apple 홈
```

## 확장 지점

1. `internal/openmiio/products.go`에 제품 ID, 모델, 유형, 메타데이터와 디코더
   함수를 등록합니다.
2. `internal/openmiio/decoders.go`에 디코더를 추가합니다. 제품군이 크면 별도
   디코더 파일을 만듭니다. 길이와 값 범위를 검증하고 `openmiio.Update`의
   타입이 있는 필드를 채워야 합니다.
3. HomeKit 서비스 구성이 같으면 기존 `ProductKind`를 재사용합니다. 새로운
   구성이면 `internal/bridge`에 `DeviceAccessory` 구현을 추가하고
   `accessoryFactories`에 생성자를 등록합니다.
4. 실제 패킷 파서, 상태 전환, 중복·역순 프레임 및 안정적인 AID 이관 테스트를
   추가합니다.
5. 한국어와 영어 프로토콜, README 및 변경 이력을 함께 갱신합니다.

`config.Device`와 `devices.json`은 특정 제품에 종속되지 않습니다. MAC/DID
매칭과 저장된 AID를 모든 제품이 공유합니다. 제품 ID가 없는 기존 항목은 계속
PDID 5860으로 이관되며, 알려진 모델이 있으면 해당 PDID를 찾습니다.

## 현재 제품군

| 유형 | 제품 | 디코더 | HomeKit 액세서리 |
|---|---|---|---|
| `climate` | `miaomiaoce.sensor_ht.o2`, PDID 5860 | 온도, 습도, 배터리 | 온도 센서 + 습도 센서 + 배터리 |
| `toothbrush` | `k0918.toothbrush.t700i`, PDID 6032 | 상태, timestamp, 점수, 배터리 | 동작 센서 + 배터리 |

## 설계 규칙

- 알 수 없는 PDID는 자동 등록 전에 거부합니다.
- Xiaomi 이벤트 timestamp는 신뢰되지 않은 입력으로 취급하고, 세션 복구에
  필요하면 게이트웨이 수신 시각을 함께 보존합니다.
- 전송 중복 제거는 제품과 독립적으로 유지합니다. 제품별 중복 예외는 좁게
  제한합니다. T700i는 동일한 시작 프레임에서 이미 활성 상태인 watchdog만
  갱신합니다.
- 표준 HomeKit 서비스를 우선합니다. 지원되지 않는 메타데이터 때문에 쓰기
  가능한 가짜 스위치나 의미가 다른 특성을 만들지 않습니다.
- 제품 지원을 추가하면서 저장된 AID를 변경하지 않습니다. 기존 Apple 홈의
  방, 이름 및 자동화가 이 식별자에 의존합니다.
- 장치별 상시 goroutine을 피하고 `Close`에서 타이머를 중지하여 저메모리
  MGL03 런타임이 정상 종료되게 합니다.

## 검증 체크리스트

```text
go test ./...
go vet ./...
python -m unittest discover -s tests -v
./scripts/build.ps1 -Version <version>
```

MIPSLE 바이너리가 CI 크기 제한 이내인지, 기존 온습도 센서 AID가 유지되는지,
새 제품이 유효한 디코딩 보고 후에만 자동 등록되는지, 업그레이드 후 Apple 홈
브리지 페어링이 유지되는지도 확인합니다.
