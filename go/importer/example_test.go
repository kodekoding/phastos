package importer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/jmoiron/sqlx"

	"github.com/kodekoding/phastos/v2/go/api"
	"github.com/kodekoding/phastos/v2/go/importer"
)

// =============================================================================
// Helper: simulasi mendapatkan file + ext dari http.Request
// =============================================================================

func getFileFromRequest(r *http.Request) (multipart.File, string) {
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Fatal(err)
	}
	ext := filepath.Ext(header.Filename) // ".csv", ".xlsx", ".xls"
	return file, ext
}

// =============================================================================
// 1. Standard Import CSV — dengan processFn + DB Transaction
//    Setiap row di-parse ke struct, di-validate, lalu di-process via processFn
// =============================================================================

// struct tujuan: field tag "csv" harus cocok dengan header di file CSV
type Employee struct {
	Name  string `json:"Name" csv:"Name" validate:"required"`
	Email string `json:"Email" csv:"Email" validate:"required,email"`
	Role  string `json:"Role" csv:"Role"`
}

func Example_importCSVWithProcess() {
	// === Setup (dalam handler API) ===
	var file multipart.File // dari r.FormFile("file")
	var trx interface{}     // database.Transactions dari app

	// Definisikan processFn: logic yang dijalankan per-row
	processFn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, workerIndex int) *api.HttpError {
		emp := singleData.(*Employee)
		fmt.Printf("[Worker-%d] Processing: %s (%s)\n", workerIndex, emp.Name, emp.Email)

		// contoh: insert ke DB
		// _, err := tx.ExecContext(ctx, "INSERT INTO employees (name, email, role) VALUES (?, ?, ?)", emp.Name, emp.Email, emp.Role)
		// if err != nil {
		//     return api.NewErr(api.WithErrorMessage(err.Error()), api.WithErrorData(map[string]any{"data": emp}))
		// }
		return nil
	}

	// === Inisialisasi Importer ===
	imp := importer.New(
		importer.WithFile(file),
		importer.WithExtFile(".csv"),               // atau ".xlsx", ".xls"
		importer.WithStructDestination(Employee{}), // struct tujuan parsing
		importer.WithTransaction(trx.(interface {
			Begin() (*sqlx.Tx, error)
			Finish(*sqlx.Tx, error)
		})),
		importer.WithProcessFn(processFn),
		importer.WithWorker(5), // 5 concurrent workers
		importer.WithProcessName("Employee Import"),
		importer.WithCtx(context.Background()),
		importer.WithSentNotifToSlack(true), // opsional: kirim notifikasi ke Slack
	)

	// === Jalankan Import ===
	result := imp.ProcessData()
	if result == nil {
		log.Fatal("import failed: validation error")
	}

	// === Ambil Hasil Import ===
	fmt.Printf("Total Data    : %d\n", result.TotalData)
	fmt.Printf("Total Success : %d\n", result.TotalSuccess)
	fmt.Printf("Total Failed  : %d\n", result.TotalFailed)
	fmt.Printf("Execution Time: %.2f seconds\n", result.ExecutionTime)

	// --- Akses data yang BERHASIL ---
	// result.SuccessList = []map[string]any
	// Setiap map key-nya = header kolom dari file
	for i, row := range result.SuccessList {
		fmt.Printf("Success[%d]: Name=%s, Email=%s, Role=%s\n",
			i, row["Name"], row["Email"], row["Role"])
	}

	// --- Akses data yang GAGAL ---
	// result.FailedList = map[string][]any
	// Key = error message, Value = list data yang gagal
	for errMsg, dataList := range result.FailedList {
		fmt.Printf("Error: %s (%d data)\n", errMsg, len(dataList))
		for _, d := range dataList {
			errData, _ := json.Marshal(d)
			fmt.Printf("  - %s\n", string(errData))
		}
	}

	// --- Kirim hasil ke client sebagai JSON response ---
	// return api.NewResponse().SetData(result)
	_ = result
}

// =============================================================================
// 2. Standard Import Excel (.xlsx) — dengan custom sheet name
// =============================================================================

func Example_importExcelXlsx() {
	var file multipart.File
	var trx interface{}

	processFn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		emp := singleData.(*Employee)
		fmt.Printf("Importing: %s\n", emp.Name)
		return nil
	}

	imp := importer.New(
		importer.WithFile(file),
		importer.WithExtFile(".xlsx"),
		importer.WithStructDestination(Employee{}),
		importer.WithTransaction(trx.(interface {
			Begin() (*sqlx.Tx, error)
			Finish(*sqlx.Tx, error)
		})),
		importer.WithProcessFn(processFn),
		importer.WithSheetName("Data Karyawan"), // custom sheet name (opsional)
		importer.WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	fmt.Printf("Imported %d data from Excel\n", result.TotalSuccess)
}

// =============================================================================
// 3. Menggunakan Hasil Import di Handler API — Contoh lengkap
// =============================================================================

func Example_importInAPIHandler() {
	// Simulasi handler:
	// func (ctrl *Controller) ImportEmployee(req api.Request, ctx context.Context) *api.Response {
	//
	//     file, ext := getFileFromRequest(req.Original)
	//     defer file.Close()
	//
	//     imp := importer.New(
	//         importer.WithFile(file),
	//         importer.WithExtFile(ext),
	//         importer.WithStructDestination(Employee{}),
	//         importer.WithTransaction(ctrl.trx),
	//         importer.WithProcessFn(func(ctx context.Context, data interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
	//             emp := data.(*Employee)
	//             if _, err := tx.ExecContext(ctx, "INSERT INTO employees ...", emp.Name); err != nil {
	//                 return api.NewErr(
	//                     api.WithErrorMessage("failed insert"),
	//                     api.WithErrorData(map[string]any{"employee": emp.Name}),
	//                 )
	//             }
	//             return nil
	//         }),
	//         importer.WithWorker(10),
	//         importer.WithProcessName("Employee Import"),
	//         importer.WithCtx(ctx),
	//     )
	//
	//     result := imp.ProcessData()
	//     if result == nil {
	//         return api.NewResponse().SetError(errors.New("import failed"))
	//     }
	//
	//     // result.SuccessList berisi semua data yang berhasil ([]map[string]any)
	//     // result.FailedList berisi semua data yang gagal per error group
	//     return api.NewResponse().SetData(result)
	// }
}

// =============================================================================
// 4. ProcessPivotData — Import pivot file dengan processFn + DB Transaction
//    Setiap entry (key-value pivot) diproses via worker pool seperti ProcessData.
//    File: shift_sample.csv
//    Row 3: Employee ID, Employee Name, 2024-03-26, 2024-03-27, ...
//    Row 4+: 444201123, Fitri, HONS, HONS, ...
// =============================================================================

func Example_processPivotData() {
	var file multipart.File
	var trx interface{} // database.Transactions dari app

	// processFn menerima map[string]any per pivot entry
	// keys di dalam map:
	//   - key column values (pakai nama header, misal "Employee ID": "444201123")
	//   - "pivot_header": header kolom value (misal "2024-03-26")
	//   - "pivot_value":  nilai cell (misal "HONS")
	processFn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, workerIndex int) *api.HttpError {
		entry := singleData.(map[string]any)

		employeeID := entry["Employee ID"].(string)
		date := entry["pivot_header"].(string)
		shift := entry["pivot_value"].(string)

		fmt.Printf("[Worker-%d] %s on %s = %s\n", workerIndex, employeeID, date, shift)

		// contoh: insert ke DB
		// _, err := tx.ExecContext(ctx, "INSERT INTO shifts (employee_id, date, shift_code) VALUES (?, ?, ?)",
		//     employeeID, date, shift)
		// if err != nil {
		//     return api.NewErr(api.WithErrorMessage(err.Error()), api.WithErrorData(entry))
		// }
		return nil
	}

	imp := importer.New(
		// === file + type ===
		importer.WithFile(file),
		importer.WithExtFile(".csv"),
		// === processing (sama seperti ProcessData) ===
		importer.WithTransaction(trx.(interface {
			Begin() (*sqlx.Tx, error)
			Finish(*sqlx.Tx, error)
		})),
		importer.WithProcessFn(processFn),
		importer.WithWorker(5),
		importer.WithProcessName("Shift Schedule Import"),
		importer.WithCtx(context.Background()),
		// === pivot config ===
		importer.WithHeaderRowIndex(2),
		importer.WithDataStartRow(3),
		importer.WithKeyColumns([]int{0}),
		importer.WithKeySeparator(";"),
		importer.WithValueStartCol(2),
	)

	// ProcessPivotData returns *ImportResult (sama seperti ProcessData)
	result := imp.ProcessPivotData()
	if result == nil {
		log.Fatal("pivot import failed")
	}

	fmt.Printf("Total Data   : %d\n", result.TotalData)
	fmt.Printf("Total Success: %d\n", result.TotalSuccess)
	fmt.Printf("Total Failed : %d\n", result.TotalFailed)

	// SuccessList berisi entry yang berhasil
	for i, row := range result.SuccessList {
		fmt.Printf("Success[%d]: Employee=%s, Date=%s, Shift=%s\n",
			i, row["Employee ID"], row["pivot_header"], row["pivot_value"])
	}

	// FailedList berisi entry yang gagal (per error group)
	for errMsg, dataList := range result.FailedList {
		fmt.Printf("Error: %s (%d data)\n", errMsg, len(dataList))
	}
}

// =============================================================================
// 5. ProcessPivotData di handler API — contoh lengkap
// =============================================================================

func Example_processPivotInAPIHandler() {
	// func (ctrl *Controller) ImportShift(req api.Request, ctx context.Context) *api.Response {
	//
	//     file, ext := getFileFromRequest(req.Original)
	//     defer file.Close()
	//
	//     imp := importer.New(
	//         importer.WithFile(file),
	//         importer.WithExtFile(ext),
	//         importer.WithTransaction(ctrl.trx),
	//         importer.WithProcessFn(func(ctx context.Context, data interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
	//             entry := data.(map[string]any)
	//             _, err := tx.ExecContext(ctx, "INSERT INTO shifts ...",
	//                 entry["Employee ID"], entry["pivot_header"], entry["pivot_value"])
	//             if err != nil {
	//                 return api.NewErr(api.WithErrorMessage(err.Error()), api.WithErrorData(entry))
	//             }
	//             return nil
	//         }),
	//         importer.WithWorker(10),
	//         importer.WithProcessName("Shift Import"),
	//         importer.WithCtx(ctx),
	//         importer.WithHeaderRowIndex(2),
	//         importer.WithDataStartRow(3),
	//         importer.WithKeyColumns([]int{0}),
	//         importer.WithValueStartCol(2),
	//     )
	//
	//     result := imp.ProcessPivotData()
	//     if result == nil {
	//         return api.NewResponse().SetError(errors.New("import failed"))
	//     }
	//     return api.NewResponse().SetData(result)
	// }
}

// =============================================================================
// 6. ReadPivotData — Baca pivot data tanpa processing (read-only)
//    Return: *PivotReadResult (Data map, Headers, MetaRows)
// =============================================================================

func Example_readPivotCSV() {
	var file multipart.File

	imp := importer.New(
		importer.WithFile(file),
		importer.WithExtFile(".csv"),
		importer.WithHeaderRowIndex(2),
		importer.WithDataStartRow(3),
		importer.WithKeyColumns([]int{0}),
		importer.WithKeySeparator(";"),
		importer.WithValueStartCol(2),
	)

	result, err := imp.ReadPivotData()
	if err != nil {
		log.Fatal(err)
	}

	// result.MetaRows = metadata rows sebelum header
	// result.Headers  = header row
	// result.Data     = map["444201123;2024-03-26"] = "HONS"
	fmt.Println("Location:", result.MetaRows[0][1])
	fmt.Println(result.Data["444201123;2024-03-26"])
	fmt.Printf("Total entries: %d\n", len(result.Data))
}

// =============================================================================
// 7. ReadPivotData dari Excel (.xlsx) + multi-key
// =============================================================================

func Example_readPivotXlsx() {
	var file multipart.File

	imp := importer.New(
		importer.WithFile(file),
		importer.WithExtFile(".xlsx"),
		importer.WithSheetName("Shift Schedule"),
		importer.WithHeaderRowIndex(2),
		importer.WithDataStartRow(3),
		importer.WithKeyColumns([]int{0, 1}), // multi-key: Employee ID + Name
		importer.WithKeySeparator(";"),
		importer.WithValueStartCol(2),
	)

	result, err := imp.ReadPivotData()
	if err != nil {
		log.Fatal(err)
	}

	// key: "444201123;Fitri Yuni Ariyanti;2024-03-26"
	fmt.Printf("Total entries: %d\n", len(result.Data))
}

// =============================================================================
// 8. ReadPivotData Streaming — OnPivotEntry callback untuk data sangat besar
// =============================================================================

func Example_readPivotStreaming() {
	var file multipart.File

	imp := importer.New(
		importer.WithFile(file),
		importer.WithExtFile(".csv"),
		importer.WithHeaderRowIndex(2),
		importer.WithDataStartRow(3),
		importer.WithKeyColumns([]int{0}),
		importer.WithValueStartCol(2),
		importer.WithOnPivotEntry(func(key, value string) {
			fmt.Printf("%s = %s\n", key, value)
		}),
	)

	result, err := imp.ReadPivotData()
	if err != nil {
		log.Fatal(err)
	}

	// result.Data == nil (OnPivotEntry aktif)
	fmt.Println("Headers:", result.Headers)
}

// =============================================================================
// 9. Menggunakan helper publik: GetDataFromXlsx / GetDataFromXls
//    Untuk baca raw data tanpa import pipeline
// =============================================================================

func Example_readRawExcelData() {
	var file multipart.File

	// Baca semua rows dari xlsx (return [][]string)
	rows, err := importer.GetDataFromXlsx(file, "Sheet1")
	if err != nil {
		log.Fatal(err)
	}

	if len(rows) == 0 {
		fmt.Println("empty sheet")
		return
	}

	headers := rows[0]
	fmt.Println("Headers:", headers)

	// Iterasi data rows
	for i := 1; i < len(rows); i++ {
		for colIdx, cell := range rows[i] {
			fmt.Printf("%s: %s  ", headers[colIdx], cell)
		}
		fmt.Println()
	}
}
