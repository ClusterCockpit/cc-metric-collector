# План решения: RawDataAggregator

## Анализ проблем в текущей реализации `interval_aggregates`

### 1. Критический баг: отправка nil в кэш
В `metricRouter.go` строка 256-258:
```go
if r.config.NumCacheIntervals > 0 {
    r.cache.Add(m)  // m может быть nil, если метрика отброшена!
}
```
Если `ProcessMessage` возвращает `nil` (метрика отброшена), `cache.Add(nil)` вызовет панику.

### 2. Баг в GetPeriod (неправильный расчет индекса)
В `metricCache.go` строка 166-168:
```go
if pindex < 0 {
    pindex = c.numPeriods - pindex  // BUG! Должно быть c.numPeriods + pindex
}
```
При `curPeriod=0, index=1` получаем `pindex = numPeriods + 1` - выход за границы массива.

### 3. Гонка данных (race condition)
В `metricCache.Start()`:
```go
c.lock.Lock()
old := rotate(tick)
starttime, endtime, metrics := c.GetPeriod(old)
c.lock.Unlock()
if len(metrics) > 0 {
    c.aggEngine.Eval(starttime, endtime, metrics)  // Eval вне lock!
}
```
`Eval()` выполняется без блокировки, но `metrics` - это срез из массива, который может быть модифицирован в `Add()`.

### 4. Неправильная очистка периода
В `rotate()`:
```go
c.intervals[oldPeriod].numMetrics = 0  // Очищает счетчик, но не срез metrics
```
При следующем `Add()` старые метрики могут быть перезаписаны, но срез `metrics` в `GetPeriod()` может все еще использоваться в `Eval()`.

### 5. `break` вместо `continue` в Eval
В `metricAggregator.go` строка 222-224:
```go
if err != nil {
    cclog.ComponentErrorf(...)
    break  // Должно быть continue!
}
```
Ошибка в одной функции прерывает обработку всех остальных функций.

### 6. Проблема с `<copy>` в meta
В router.json есть `"unit": "<copy>"`, но в `copy_meta` при значении не `<copy>` присваивается исходное значение, что правильно. Однако в `copy_tags` и `copy_meta` нет обработки случая, когда тег/мета отсутствует в метриках.

---

## План решения

Создам новый компонент **`RawDataAggregator`** с новой опцией конфигурации **`raw_aggregates`**, который:

1. **Правильно накапливает сырые данные** - использует отдельный буфер для каждой агрегации
2. **Исправляет все перечисленные баги** - правильная синхронизация, копирование данных, обработка ошибок
3. **Имеет тот же интерфейс конфигурации** что и `interval_aggregates`

### Структура решения:

**Новый файл: `internal/metricRouter/rawDataAggregator.go`**
- `RawDataAggregator` - новый компонент, который:
  - Получает метрики из router (через channel или напрямую)
  - Накапливает сырые данные в буфере (с правильной синхронизацией)
  - Раз в `interval` вычисляет агрегаты
  - Отправляет агрегированные метрики в output channel

**Изменения в `metricRouter.go`**:
- Добавить новую опцию `RawAgg` в `metricRouterConfig`
- Инициализировать `RawDataAggregator` при наличии `raw_aggregates` в конфиге
- Подключить его к потоку метрик

**Изменения в `metricAggregator.go`**:
- Исправить `break` на `continue` в `Eval()` (этот баг влияет и на существующий код)

### Конфигурация (router.json):
```json
{
  "raw_aggregates": [
    {
      "name": "l2_cache_misses_min",
      "if": "metric.Name() == 'l2_cache_misses'",
      "function": "min(values)",
      "tags": {"stat": "min", "type": "node"},
      "meta": {"group": "CacheMisses", "unit": "count", "source": "LikwidCollector"}
    }
  ]
}
```

Интерфейс полностью совместим с `interval_aggregates`.

---

## Задачи

- [ ] Создать rawDataAggregator.go с новой реализацией
- [ ] Добавить опцию raw_aggregates в metricRouterConfig
- [ ] Интегрировать RawDataAggregator в metricRouter
- [ ] Исправить баг с break/continue в metricAggregator.go
- [ ] Протестировать работу новой опции