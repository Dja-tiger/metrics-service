# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

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
