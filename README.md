# metrics-service

Сервис сбора метрик и алертинга: агент собирает метрики и передаёт их серверу.
Проект основан на учебном шаблоне Yandex Practicum.

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. Установите Go версии, указанной в `go.mod`, или новее. Модуль уже создан;
   повторный `go mod init` не нужен.
3. Из корня проекта загрузите зависимости и запустите тесты:

```bash
go mod download
go test ./...
```

Запуск в двух отдельных терминалах:

```bash
go run ./cmd/server
go run ./cmd/agent
```

Все команды ниже выполняются из корня репозитория, если не указано иначе.
Пути к файлам относительные и не зависят от расположения проекта на компьютере.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Шифрование запросов

Агент принимает путь к публичному RSA-ключу через `-crypto-key` или `CRYPTO_KEY`.
Сервер использует те же параметры для пути к приватному ключу. Переменная окружения
имеет приоритет над флагом, в том числе пустое значение. Без пути шифрование отключено.
Ключи загружаются при запуске; неверный путь, формат или тип ключа останавливает запуск.

Создание тестовых ключей в исключённом из Git каталоге `.local/keys`
(не добавляйте приватный ключ в Git):

```bash
mkdir -p .local/keys
chmod 700 .local/keys
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out .local/keys/metrics-private.pem
chmod 600 .local/keys/metrics-private.pem
openssl pkey -in .local/keys/metrics-private.pem -pubout -out .local/keys/metrics-public.pem
```

Запустите в отдельных терминалах:

```bash
go run ./cmd/server -crypto-key=.local/keys/metrics-private.pem
go run ./cmd/agent -crypto-key=.local/keys/metrics-public.pem
```

Поддерживаются PEM-ключи RSA от 2048 бит: публичные PKIX/PKCS#1 и приватные
PKCS#8/PKCS#1 без пароля. Пакет `internal/encryption` использует RSA-OAEP-SHA256
для шифрования случайного ключа AES-256-GCM. Это позволяет шифровать большие батчи,
которые не помещаются в одно сообщение RSA. Ключ AES и nonce создаются заново
для каждого батча. Изменённый шифротекст отклоняется.

Порядок обработки: JSON → gzip → шифрование; на сервере — расшифровка → gzip →
проверка `HashSHA256` исходного JSON → хендлер. Запрос отмечается заголовком
`X-Metrics-Encryption: rsa-oaep-aes256-gcm-v1`; `Content-Encoding: gzip` описывает
данные внутри зашифрованного тела. Бинарное тело содержит байт версии `1`,
RSA-зашифрованный ключ (размер модуля RSA), nonce GCM (12 байт) и шифротекст
с тегом GCM (16 байт). Версия и зашифрованный ключ включаются в AAD.

Для совместимости сервер принимает и незашифрованные запросы без этого заголовка.
Ошибки расшифровки, неизвестный алгоритм и зашифрованный запрос без настроенного
приватного ключа возвращают HTTP 400. Размер зашифрованного тела ограничен 16 MiB.
Ответы сервера не шифруются; gzip и подпись ответов продолжают работать.

## Информация о сборке

Агент и сервер при запуске выводят в stdout версию, дату и коммит сборки.
Если значение не задано, выводится `N/A`. Значения передаются через `-ldflags`:

```bash
go build -ldflags "-X main.buildVersion=v1.0.0 -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.buildCommit=$(git rev-parse HEAD)" -o ./cmd/server/server ./cmd/server
go build -ldflags "-X main.buildVersion=v1.0.0 -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.buildCommit=$(git rev-parse HEAD)" -o ./cmd/agent/agent ./cmd/agent
```

## Пул объектов

Пакет `internal/pool` содержит `Pool[T interface{ Reset() }]` на основе `sync.Pool`.
Конструктор принимает фабрику новых объектов; `Get()` возвращает объект, а `Put()`
вызывает его `Reset()` перед возвратом в пул:

```go
p := pool.New(func() *models.Metrics { return &models.Metrics{} })
metric := p.Get()
metric.ID = "Alloc"
// Работа с метрикой завершена.
p.Put(metric)
```

При `nil`-фабрике `Get()` из пустого пула возвращает нулевое значение типа `T`.
Непустая фабрика должна быть безопасна для одновременных вызовов и возвращать новый ненулевой
объект. После `Put()` нельзя обращаться к объекту или возвращать его повторно.
Сам пул нельзя копировать после начала использования. `sync.Pool` может удалять
объекты при сборке мусора, поэтому повторное получение того же объекта не гарантируется.

```bash
go test -race ./internal/pool -count=1
```

## Генерация Reset

Добавьте комментарий `// generate:reset` непосредственно над объявлением структуры
и запустите из любой директории текущего Go-модуля:

```bash
go run ./cmd/reset
```

Команда выше предназначена для запуска из корня; из вложенной директории укажите
соответствующий относительный путь к `cmd/reset`. Утилита сама находит `go.mod`
и сканирует пакеты модуля через `./...` с учётом текущих build tags и платформы.
Тестовые файлы, `testdata`, `vendor` и вложенные модули не включаются.

Методы каждого пакета записываются в его `reset.gen.go`. Повторный запуск обновляет
этот файл. В проекте генерация включена для структуры `Metrics`.

- Примитивы сбрасываются в нулевые значения, слайсы обрезаются до нулевой длины,
  мапы очищаются через `clear`.
- Ненулевые указатели сохраняются, сбрасывается их содержимое; `nil` остаётся `nil`.
- Вложенные структуры вызывают свой `Reset()`, если он есть, иначе обнуляются целиком.
- Элементы массивов сбрасываются по тем же правилам; интерфейсы, функции и каналы
  обнуляются. Параметры типов обнуляются согласно их конкретному типу.

Рекурсивный сброс рассчитан на структуры без циклических ссылок. Во время сброса
нельзя одновременно изменять тот же объект из другой горутины. Для `Metrics`
сброс сохраняет ненулевые `Delta` и `Value`, записывая в них нули.

Тесты генератора компилируют полученный код и проверяют его поведение:

```bash
go test ./cmd/reset -count=1
```

## Статический анализатор

Анализатор `cmd/linter` запускается через `singlechecker` и проверяет прямые вызовы:

- встроенной функции `panic` — запрещены во всём проекте;
- `log.Fatal`, `log.Fatalf`, `log.Fatalln` и `os.Exit` — разрешены только внутри функции `main` пакета `main`.

Вложенные анонимные функции считаются отдельными функциями. Псевдонимы импортов
и dot-import поддерживаются; одноимённые пользовательские функции и методы
`testing.T` не вызывают предупреждений. Вызовы через переменные-функции не отслеживаются.

Проверка проекта:

```bash
go run ./cmd/linter ./...
```

Тесты анализатора используют `analysistest` и примеры с ожидаемыми диагностическими
сообщениями в `cmd/linter/testdata`:

```bash
go test ./cmd/linter -count=1
```

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Профилирование памяти

Для инкремента 17 добавлены benchmark-тесты для основных компонентов сервиса:

- сохранение и batch-обновление метрик в `MemStorage`;
- обработка batch JSON-запроса `POST /updates/`;
- отправка batch-метрик агентом;
- snapshot метрик агента;
- audit-наблюдатели;
- расчёт и проверка SHA256-подписи.

Базовый профиль памяти сохранён в `profiles/base.pprof`, итоговый профиль после оптимизации — в `profiles/result.pprof`.

Команды для снятия профилей:

```bash
go test ./internal/repository -run=^$ -bench=BenchmarkMemStorageSaveToFile -benchmem -memprofile=profiles/base.pprof
go test ./internal/repository -run=^$ -bench=BenchmarkMemStorageSaveToFile -benchmem -memprofile=profiles/result.pprof
pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

До оптимизации сохранение метрик формировало полный pretty JSON в памяти через `json.MarshalIndent`, а затем записывало его в файл. После анализа `pprof top`, `pprof list` и `pprof peek` сохранение переведено на запись компактного JSON через `json.Encoder`, а создание snapshot-метрик уменьшает количество отдельных heap-аллокаций для значений `Value` и `Delta`.

Ниже сохранены исторические результаты инкремента 17, а не измерение текущей версии.
Имена `github.com/Dja-tiger/metrics-service/...` в выводе pprof — пути Go-пакетов,
а не абсолютные пути на компьютере разработчика.

Результат benchmark для `SaveToFile`:

```text
До:    717533 ns/op  255377 B/op  1044 allocs/op
После: 405814 ns/op   78191 B/op    19 allocs/op
```

Результат сравнения профилей:

```text
File: repository.test
Type: alloc_space
Time: 2026-07-17 13:29:12 MSK
Showing nodes accounting for -325.40MB, 49.07% of 663.09MB total
Dropped 68 nodes (cum <= 3.32MB)
      flat  flat%   sum%        cum   cum%
 -290.59MB 43.82% 43.82%  -459.93MB 69.36%  encoding/json.MarshalIndent
 -139.35MB 21.02% 64.84%  -168.84MB 25.46%  encoding/json.Marshal
  122.60MB 18.49% 46.35%   122.60MB 18.49%  github.com/Dja-tiger/metrics-service/internal/repository.(*MemStorage).snapshotMetrics
  -16.06MB  2.42% 48.77%   -16.06MB  2.42%  bytes.growSlice
   -2.50MB  0.38% 49.15%    -2.50MB  0.38%  sync.(*Pool).pinSlow
    0.50MB 0.075% 49.07%  -322.40MB 48.62%  github.com/Dja-tiger/metrics-service/internal/repository.(*MemStorage).SaveToFile
         0     0% 49.07%    -6.11MB  0.92%  bytes.(*Buffer).Write
         0     0% 49.07%   -10.45MB  1.58%  bytes.(*Buffer).WriteString
         0     0% 49.07%   -16.06MB  2.42%  bytes.(*Buffer).grow
         0     0% 49.07%    11.43MB  1.72%  encoding/json.(*Encoder).Encode
         0     0% 49.07%   -16.06MB  2.42%  encoding/json.(*encodeState).marshal
         0     0% 49.07%   -16.06MB  2.42%  encoding/json.(*encodeState).reflectValue
         0     0% 49.07%   -16.06MB  2.42%  encoding/json.arrayEncoder.encode
         0     0% 49.07%    -2.08MB  0.31%  encoding/json.ptrEncoder.encode
         0     0% 49.07%   -16.06MB  2.42%  encoding/json.sliceEncoder.encode
         0     0% 49.07%    -4.03MB  0.61%  encoding/json.stringEncoder
         0     0% 49.07%   -16.56MB  2.50%  encoding/json.structEncoder.encode
         0     0% 49.07%  -321.38MB 48.47%  github.com/Dja-tiger/metrics-service/internal/repository.BenchmarkMemStorageSaveToFile
         0     0% 49.07%    -2.50MB  0.38%  sync.(*Pool).pin
         0     0% 49.07%  -321.91MB 48.55%  testing.(*B).launch
         0     0% 49.07%  -321.88MB 48.54%  testing.(*B).runN
```
