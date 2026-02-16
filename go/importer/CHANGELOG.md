# Changelog - Importer Package

## [Unreleased] - 2026-02-15

### Added
- `ImportResult.SuccessList` (`[]map[string]any`) — mengembalikan semua data yang berhasil diproses ke caller. Setiap entry adalah map dengan key = nama header kolom dari file.
- `ImportResult.TotalSuccess` (`int`) — jumlah data yang berhasil diproses.
- Internal type `rowData` — membawa parsed struct dan raw map data melalui channel dari file reader ke worker.
- Internal type `processedResult` — membawa raw data dan error dari worker ke aggregator.
- Helper function `buildRowMap()` — membuat map baru per row dari header dan cell values.

### Changed

#### CSV Streaming (`csv.go`)
- **Breaking (internal):** `readFromCSV` sekarang menggunakan `csvReader.Read()` row-by-row, menggantikan `csvReader.ReadAll()` yang memuat seluruh file ke memory.
- Channel type berubah dari `<-chan interface{}` ke `<-chan rowData`.
- Setiap row membuat map baru (tidak lagi menggunakan shared `mapContent`).

#### Excel Streaming (`excel.go`)
- **xlsx:** `readFromExcel` sekarang menggunakan `excelize.Rows()` streaming iterator (`rows.Next()` + `rows.Columns()`), menggantikan `GetRows()` yang memuat seluruh sheet ke memory.
- **xls:** Row diproses langsung dari `sheetData.Row()` ke channel tanpa mengumpulkan ke intermediate `[][]string` terlebih dahulu.
- Fungsi publik `GetDataFromXlsx()` dan `GetDataFromXls()` tetap dipertahankan untuk backward compatibility.

#### Processing (`import.go`)
- `processEachData` sekarang menerima `<-chan rowData` dan mengembalikan `<-chan *processedResult` (sebelumnya `<-chan interface{}` dan `<-chan *api.HttpError`).
- `processData` mengumpulkan data sukses ke `SuccessList` dan menghitung `TotalSuccess`.

### Fixed
- **Bug shared `mapContent`:** Sebelumnya, satu map di-reuse untuk semua row. Jika sebuah row memiliki kolom lebih sedikit dari header, data dari row sebelumnya akan bocor (stale values). Sekarang setiap row membuat map sendiri via `buildRowMap()`.

### Removed
- Field `mapContent` dari struct `importer` (tidak diperlukan lagi).
- Parameter `mapContent` dari `readFromCSV` dan `readFromExcel`.
- Dependency ke `helper.ConvertStructToMap()` di `WithStructDestination`.
