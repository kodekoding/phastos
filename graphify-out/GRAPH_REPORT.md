# Graph Report - phastos  (2026-08-03)

## Corpus Check
- 265 files · ~269,617 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 4873 nodes · 12213 edges · 229 communities (197 shown, 32 thin omitted)
- Extraction: 72% EXTRACTED · 28% INFERRED · 0% AMBIGUOUS · INFERRED: 3422 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `c3827afb`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- sql_deep_test.go
- otel.go
- mail_coverage_test.go
- Context
- NewSlackApp
- newSQLWithStubs
- generator_test.go
- New
- App
- FastRequest
- runNetHTTPBenchmark
- Call
- helper_coverage_test.go
- NewRateLimiter
- WithAPITimeout
- Platform
- generator_coverage_test.go
- createMultipartFile
- Call
- stubBucket
- SQL
- Database / Query Builder
- stubSQL
- database/pool_test.go
- API Framework
- sql_test.go
- FastNewRateLimiter
- slack/api_test.go
- NewApp
- sql_helpers_test.go
- request_test.go
- struct_rows_test.go
- hub_test.go
- fasthttp_app_test.go
- action_test.go
- GenerateSelectCols
- Context
- ServiceEnv
- sse_test.go
- phastos_fasthttp_test.go
- Hub
- fasthttp_coverage_test.go
- NewCSV
- generator/example_test.go
- .getBaseQuery
- fasthttp_app.go
- Phastos SSE
- NewBulkGenerator
- Value
- WithDataStartRow
- NewResponse
- handler/api.go
- google
- stubNotifAction
- truncateTable
- lruStmtCache
- BeginTrxFromContext
- .Wrap
- response/response_test.go
- MockISQL
- buildPivotEntries
- null/struct_test.go
- httpStub
- stubStore
- New
- Helpers
- struct_extended_test.go
- ClientDeliveryManager
- WithExtFile
- Stmt
- database/sql.go
- mockStorage
- NewBaseWrite
- isql.go
- telegram_test.go
- null/struct.go
- scheduler_coverage_test.go
- controller.go
- Response
- Time
- placeholders
- monitor_test.go
- Monitoring (Observability)
- UploadToTmp
- MockMaster
- WithTimeout
- .ProcessData
- WithTimeZone
- NewTransaction
- readPivot
- excel
- Request
- wraphandler_test.go
- fast_id_test.go
- Banner
- slack
- InitHandler
- WaitTermSig
- request_ctx_test.go
- stubNotifAction
- SSE Token Authentication Guide
- stubPlatform
- helper/struct_test.go
- MockFollowerMockRecorder
- helper.go
- Notifications
- entity/slack/slack_test.go
- CryptoManager
- utils.go
- Basic Operations
- Test_nullValidator
- GetBuffer
- File Structure
- FlattenJSONB
- echo_test.go
- serveFastHTTPs
- fiber_test.go
- serveHTTPs
- beego_test.go
- newStubConn
- Service
- PDF
- storage/definition_test.go
- init_test.go
- Service
- .WrapToContext
- generateUser
- xorm_test.go
- Conn
- string_case.go
- string_generator.go
- template_test.go
- compositeProvider
- TestGoogleStorageExtended
- Action
- New
- .buildOperation
- errors.go
- actionFakeConn
- checkInitiateWhere
- entity_test.go
- csv
- Global Constraints
- OpenAPI Docs — Auto-Generated API Documentation for Phastos
- HandlerFunc
- NewInt8
- NewErr
- SetNotif
- NewUint32
- NewUint8
- ISQL
- Design
- Route Group — Nested Group Routes for Phastos Controller
- sqlTestFakeStmt
- FastJWTAuth
- NewFloat64
- controller_test.go
- constants_test.go
- CreateAsyncContext
- GetNotif
- .Register
- ReliableSSEClient
- binding_extended_test.go
- cache_test.go
- TestRemove
- processing_test.go
- StaticAuth
- server/definition.go
- stubResponseWriter
- Channel
- monitor.go
- PanicHandler
- mockController
- mockControllerWithImpl
- middlewares_extended_test.go
- server/definition_test.go
- entity/context.go
- stubTransactions
- MessageBuffer
- actionFakeErrorResult
- actionFakeResult
- actionFakeTx
- TestMain
- Global Constraints
- wrap_function.go
- rowsAffectedErrorResult
- GetJWT
- errorWriter
- Options Functions
- .reset
- .reset
- .reset
- Global Constraints
- .reset
- NewInt16
- checkKeyword
- stubFlusher
- Buckets
- SetProvider
- stubAction
- notifications/init.go
- HandlerV2 on Version-Group Sub-Router: Bug Fixes
- Phastos
- PhastosFastHttpNative
- Fix Singleflight NPE in wrapHandler
- GenerateJWTToken
- Context
- NewDrive
- grpcStub
- run.sh
- API Benchmarks
- Env
- github.com/kodekoding/phastos/v2
- run_all.sh
- Fast-Path Architecture
- Reflection Caching (B1)
- pre-push
- DB Optimization Report
- Database Action Usage Example
- Importer Streaming (CSV/Excel)

## God Nodes (most connected - your core abstractions)
1. `NewApp()` - 117 edges
2. `WithTimezone()` - 109 edges
3. `New()` - 97 edges
4. `newStubConn()` - 95 edges
5. `NewResponse()` - 94 edges
6. `HandlerFunc` - 88 edges
7. `createMultipartFile()` - 85 edges
8. `newTestStore()` - 76 edges
9. `WithExtFile()` - 71 edges
10. `WithAPITimeout()` - 68 edges

## Surprising Connections (you probably didn't know these)
- `echoMiddleware1()` --references--> `HandlerFunc`  [EXTRACTED]
  benchmark/api/echo_test.go → go/cron/cron.go
- `echoMiddleware2()` --references--> `HandlerFunc`  [EXTRACTED]
  benchmark/api/echo_test.go → go/cron/cron.go
- `echoMiddleware3()` --references--> `HandlerFunc`  [EXTRACTED]
  benchmark/api/echo_test.go → go/cron/cron.go
- `setupPhastosChiApp()` --calls--> `NewApp()`  [INFERRED]
  benchmark/api/phastos_chi_test.go → go/api/app.go
- `setupPhastosChiApp()` --calls--> `WithAPITimeout()`  [INFERRED]
  benchmark/api/phastos_chi_test.go → go/api/app.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Phastos Performance Optimization Journey** — sync_handler_path, header_based_request_id, profile_guided_optimizations, generatefastid, reflection_caching, template_caching, multilayer_statement_caching, fastpath_architecture [INFERRED]
- **Phastos SSE Subsystem** — phastos_sse, sse_token_auth, sse_encrypted_token, sse_controller_structure, sse_http3_compatibility, sse_test_client, ssehub [INFERRED]

## Communities (229 total, 32 thin omitted)

### Community 0 - "sql_deep_test.go"
Cohesion: 0.05
Nodes (108): fakeNoRowsDriver, Connect(), connectDB(), DB, T, openFakeExecErrDB(), openFakeNoRowsDB(), openFakeQueryRowDB() (+100 more)

### Community 1 - "otel.go"
Cohesion: 0.20
Nodes (13): Context, Handler, TracerProvider, initOTel(), initOTelFromEnv(), InitOTelOnly(), InitOTelSDK(), IsOTelActive() (+5 more)

### Community 2 - "mail_coverage_test.go"
Cohesion: 0.07
Nodes (61): Auth, T, TestMandrillAddAttachment(), TestMandrillAddRecipient(), TestMandrillReset(), TestMandrillSendSuccess_NoTemplate(), TestMandrillSendSuccess_WithTemplate(), TestMandrillSendWithoutTemplate() (+53 more)

### Community 3 - "Context"
Cohesion: 0.25
Nodes (3): stubCaches, Context, Duration

### Community 4 - "NewSlackApp"
Cohesion: 0.06
Nodes (56): Event, EventsAPIEvent, RegisterEvent(), T, TestRegisterEvent(), TestRegisterEventWithEventType(), TestRegisterEventWithIdentifier(), TestSocketEventStruct() (+48 more)

### Community 5 - "newSQLWithStubs"
Cohesion: 0.08
Nodes (47): sqlmockResult, stubDB, Context, Result, Row, Rows, SQL, T (+39 more)

### Community 6 - "generator_test.go"
Cohesion: 0.06
Nodes (91): mockGenerator, NewExcel(), T, TestBanner_AddImageLayer(), TestBanner_AddLabel(), TestBanner_FileName(), TestBanner_Generate_WithImageLayer(), TestBanner_Generate_WithLabel_InvalidFont() (+83 more)

### Community 7 - "New"
Cohesion: 0.10
Nodes (51): buildRowMap(), New(), T, TestBuildRowMap(), TestBuildRowMap_EmptyHeaders(), TestBuildRowMap_MoreHeadersThanRow(), TestBuildRowMap_MoreRowsThanHeaders(), TestGetDataFromXls_InvalidFile() (+43 more)

### Community 8 - "App"
Cohesion: 0.05
Nodes (39): Apps, Controllers, handler2WithMeta, HandlerV2Meta, newRelicHandlerWrapper, Options, otelHandlerWrapper, PathParamType (+31 more)

### Community 9 - "FastRequest"
Cohesion: 0.22
Nodes (3): FastRequest, RequestCtx, matchParamPath()

### Community 10 - "runNetHTTPBenchmark"
Cohesion: 0.07
Nodes (49): JSONResponse, phastosChiJSONController, phastosChiMiddlewareController, phastosChiPathParamController, phastosChiQueryParamController, BenchmarkChi_JSON(), BenchmarkChi_Middleware(), BenchmarkChi_PathParam() (+41 more)

### Community 11 - "Call"
Cohesion: 0.08
Nodes (14): Call, Context, Controller, Result, Row, Rows, Tx, TxOptions (+6 more)

### Community 12 - "helper_coverage_test.go"
Cohesion: 0.08
Nodes (48): T, TestComputeUpdateTemplate_EmbeddedStruct(), TestConstructColNameAndValue_PointerStruct(), TestConstructColNameAndValueBulk_DifferentLengths(), TestConstructColNameAndValueForUpdate_MoreCases(), TestConvertStructToMap_ExcelTag(), TestDecrypt_ShortCiphertext(), TestGenerateBulk_WithCondition() (+40 more)

### Community 13 - "NewRateLimiter"
Cohesion: 0.19
Nodes (25): defaultKeyExtractor(), Handler, Limiter, NewRateLimiter(), T, TestDefaultKeyExtractor_RemoteAddr(), TestDefaultKeyExtractor_RemoteAddrNoPort(), TestDefaultKeyExtractor_XForwardedFor() (+17 more)

### Community 14 - "WithAPITimeout"
Cohesion: 0.09
Nodes (56): handler2TestPayload, HandlerV2, T, TestApp_AddController_InjectsMiddlewares(), TestApp_flushPendingMiddlewares_WithGlobalMiddlewares(), TestApp_flushPendingMiddlewares_WithoutVersionPrefix(), TestApp_Init_InvalidSingleflightEnv(), TestApp_Init_SingleflightActive() (+48 more)

### Community 15 - "Platform"
Cohesion: 0.25
Nodes (3): Context, Handler, Platform

### Community 16 - "generator_coverage_test.go"
Cohesion: 0.12
Nodes (51): coverageMockGenerator, Options, NewBanner(), WithHeight(), WithWidth(), T, TestBanner_Generate_EmptyLabels(), TestBanner_Generate_WithImageLayerV2() (+43 more)

### Community 17 - "createMultipartFile"
Cohesion: 0.14
Nodes (37): readPivotFromXlsx(), GetDataFromXlsx(), T, plogGet(), TestBuildPivotEntries_EmptyHeader(), TestBuildPivotEntries_HeaderLenLessThanValueStartCol(), TestBuildPivotEntries_KeyColOutOfBounds(), TestBuildPivotEntries_WithOnEntryCallback() (+29 more)

### Community 18 - "Call"
Cohesion: 0.08
Nodes (3): Call, MockISQLMockRecorder, MockMasterMockRecorder

### Community 19 - "stubBucket"
Cohesion: 0.22
Nodes (3): Context, File, stubBucket

### Community 20 - "SQL"
Cohesion: 0.07
Nodes (24): BaseColumn, commonDB, executedQuery, Follower, Master, ResponseMetaData, SQLConfig, SQLs (+16 more)

### Community 21 - "Database / Query Builder"
Cohesion: 0.04
Nodes (48): action.Base — Combined Read + Write, Action Constants, Bulk Insert, Complete Usage Example, Connect, Connection, Core Types, CRUD Helpers (+40 more)

### Community 22 - "stubSQL"
Cohesion: 0.08
Nodes (12): actionFakeErrorStmt, actionFakeStmt, stubSQL, Context, CUDResponse, QueryOpts, Result, Row (+4 more)

### Community 23 - "database/pool_test.go"
Cohesion: 0.11
Nodes (43): GetCUDConstructData(), GetCUDResponse(), GetQueryOpts(), GetSelectResponse(), GetTableRequest(), CUDConstructData, CUDResponse, CUDConstructData (+35 more)

### Community 24 - "API Framework"
Cohesion: 0.04
Nodes (46): Adding Controllers, API Framework, Builder Methods, Built-in Middleware, Controller Interface, ControllerImpl, Controllers & Routes, Convenience Constructors (+38 more)

### Community 25 - "sql_test.go"
Cohesion: 0.07
Nodes (39): sqlTestFakeConn, sqlTestFakeDriver, sqlTestFakeResult, sqlTestFakeTx, DB, SQL, newSQL(), Conn (+31 more)

### Community 26 - "FastNewRateLimiter"
Cohesion: 0.13
Nodes (26): FastKeyExtractor, fastRateLimiter, FastRateLimiterOption, Map, Unauthorized(), fastDefaultKeyExtractor(), fastJWTUnauthorized(), FastNewRateLimiter() (+18 more)

### Community 27 - "slack/api_test.go"
Cohesion: 0.12
Nodes (33): NewSlack(), Response, Server, T, newSlackWithTestServer(), TestAddReminderToChannel(), TestAddReminderToChannel_Error(), TestArchiveChannel() (+25 more)

### Community 28 - "NewApp"
Cohesion: 0.13
Nodes (59): NewApp(), T, TestAddGlobalMiddleware(), TestApp_AddController(), TestApp_AddController_EmptyRouteGroup(), TestApp_AddController_GroupMiddlewareInheritance(), TestApp_AddController_MixedFlatAndGroup(), TestApp_AddController_WithMiddlewares() (+51 more)

### Community 29 - "sql_helpers_test.go"
Cohesion: 0.12
Nodes (42): checkCreatedDateParam(), GenerateAddOnQuery(), T, TestCheckCreatedDateParam_CustomDateColFilter(), TestCheckCreatedDateParam_EndOnly(), TestCheckCreatedDateParam_IncludeDeleted(), TestCheckCreatedDateParam_IsDeleted(), TestCheckCreatedDateParam_IsDeletedWithMainTableAlias() (+34 more)

### Community 30 - "request_test.go"
Cohesion: 0.09
Nodes (44): httpRequestKey, pathParamsKey, queryParamsKey, requestBodyKey, testPayload, convertParam(), Context, T (+36 more)

### Community 31 - "struct_rows_test.go"
Cohesion: 0.10
Nodes (44): boolField, floatField, intField, jsonbDoc, ptrField, TestEmbeddedStruct, TestStruct, testUnTaggedStruct (+36 more)

### Community 32 - "hub_test.go"
Cohesion: 0.16
Nodes (38): NewCryptoManager(), Context, T, newStubResponseWriter(), TestBufferedMessageJSONMarshal(), TestClientSendMessage_AllFields(), TestClientSendMessage_NoEvent(), TestClientSendMessage_NoID() (+30 more)

### Community 33 - "fasthttp_app_test.go"
Cohesion: 0.12
Nodes (40): NewFastHttpApp(), NewFastResponse(), newFastRouter(), T, TestFastHttpApp_AddController(), TestFastHttpApp_AddController_EmptyRouteGroup(), TestFastHttpApp_AddController_WithDirectHandler(), TestFastHttpApp_AddController_WithMiddlewares() (+32 more)

### Community 34 - "action_test.go"
Cohesion: 0.15
Nodes (33): actionFakeDriver, actionFakeErrorDriver, testStruct, Conn, SQL, T, newRelicCtx(), newSQLForAction() (+25 more)

### Community 35 - "GenerateSelectCols"
Cohesion: 0.23
Nodes (18): TestExtractFixedUpdateValues_EmptyStruct(), TestExtractUpdateValues_NilPointer(), computeUpdateTemplate(), ExtractFixedUpdateValues(), ExtractUpdateValues(), getSelectColsFromCache(), getStructTypeInfo(), GetUpdateTemplate() (+10 more)

### Community 36 - "Context"
Cohesion: 0.09
Nodes (4): Context, Row, Rows, MockFollower

### Community 37 - "ServiceEnv"
Cohesion: 0.05
Nodes (70): Ctx(), Application, Log(), NewLog(), T, resetLogForTest(), TestANewLog_ProductionEnv(), TestCtx_NilLoggerInit() (+62 more)

### Community 38 - "sse_test.go"
Cohesion: 0.15
Nodes (32): NewClientDeliveryManager(), NewMessageBuffer(), T, NewHubWithCancel(), TestBufferedMessageStruct(), TestClientDeliveryManagerGetTracker(), TestClientDeliveryManagerRegister(), TestClientDeliveryManagerUnregister() (+24 more)

### Community 39 - "phastos_fasthttp_test.go"
Cohesion: 0.11
Nodes (28): directJSONController, directMiddlewareController, directPathParamController, FastControllerConfig, fastJSONController, fastMiddlewareController, fastPathParamController, fastQueryParamController (+20 more)

### Community 40 - "Hub"
Cohesion: 0.13
Nodes (14): CancelFunc, Flusher, TestNewSSEMessageWithData(), Context, Mutex, ResponseWriter, RWMutex, NewSSEMessage() (+6 more)

### Community 41 - "fasthttp_coverage_test.go"
Cohesion: 0.12
Nodes (34): fastGenerateUniqueRequestKey(), fastRequestLogger(), fastSendResponse(), Response, T, TestFastGenerateUniqueRequestKey(), TestFastGenerateUniqueRequestKey_WithNoQueryParams(), TestFastGenerateUniqueRequestKey_WithXForwardedFor() (+26 more)

### Community 42 - "NewCSV"
Cohesion: 0.13
Nodes (30): stubGenerator, NewCSV(), T, TestBulkGeneratorAdd(), TestBulkGeneratorDefaults(), TestBulkResult(), TestConverterOptions(), TestCSVAppendAfterError() (+22 more)

### Community 43 - "generator/example_test.go"
Cohesion: 0.11
Nodes (18): WithBackgroudColor(), Example_bannerGenerator(), Example_bulkGenerateAll(), Example_csvGenerator(), Example_excelGenerator(), Example_pdfGenerator(), Example_pdfGeneratorCustomOptions(), Example_qrGenerator() (+10 more)

### Community 44 - ".getBaseQuery"
Cohesion: 0.29
Nodes (9): BaseRead, readQueryCacheKey, selectByIdCacheEntry, getBaseQueryCached(), getSelectByIdQueryCached(), Context, QueryOpts, TableRequest (+1 more)

### Community 45 - "fasthttp_app.go"
Cohesion: 0.15
Nodes (12): FastController, FastDirectHandler, FastHandler, FastHttpApp, FastMiddleware, FastResponse, FastRoute, fastRouter (+4 more)

### Community 46 - "Phastos SSE"
Cohesion: 0.33
Nodes (7): Phastos SSE, SSE Controller Structure, SSE Encrypted Token Authentication, SSE HTTP/3 Compatibility, SSE Test Client (HTML), SSE Token Authentication, SSEHub

### Community 47 - "NewBulkGenerator"
Cohesion: 0.18
Nodes (17): BulkGenerator, BulkOption, BulkResult, NewBulkGenerator(), WithBulkWorker(), Example_bulkGenerateZip(), TestBulkGenerator_GenerateAll_SingleWorker(), TestBulkGeneratorWithWorker() (+9 more)

### Community 48 - "Value"
Cohesion: 0.09
Nodes (9): fakeExecErrStmt, fakeNoRowsRows, fakeNoRowsStmt, fakeQueryRowRows, fakeQueryRowStmt, fakeRowsAffectedErrStmt, Result, Rows (+1 more)

### Community 49 - "WithDataStartRow"
Cohesion: 0.22
Nodes (30): Example_processPivotData(), Example_readPivotCSV(), Example_readPivotStreaming(), Example_readPivotXlsx(), Example_readRawExcelData(), WithDataStartRow(), WithHeaderRowIndex(), WithKeyColumns() (+22 more)

### Community 50 - "NewResponse"
Cohesion: 0.13
Nodes (46): TestResponse_ResetClearsCustomHeader(), TestResponse_Send_WithHttpError_ExistingTraceId(), TestResponse_Send_WithHttpError_TraceIdInheritance(), TestResponse_Send_WithPaginationMetaData(), TestResponse_Send_WithSelectResponseNoMetaData(), TestResponse_SetData_WithNonSelectResponse(), TestSentNotif_NilError(), TestSentNotif_NilNotifContext() (+38 more)

### Community 51 - "handler/api.go"
Cohesion: 0.14
Nodes (24): Handler, RegisterRoute(), T, TestAPIRequestStruct(), TestControllerConfig(), TestControllerImpl(), TestControllerImplJoinMiddleware(), TestRegisterRoute() (+16 more)

### Community 52 - "google"
Cohesion: 0.23
Nodes (4): BucketHandle, Context, File, google

### Community 54 - "truncateTable"
Cohesion: 0.22
Nodes (22): BenchmarkBun_BulkInsert(), BenchmarkBun_Delete(), BenchmarkBun_Insert(), BenchmarkBun_Insert_Parallel(), BenchmarkBun_SelectByID(), BenchmarkBun_SelectByID_Parallel(), BenchmarkBun_SelectList(), BenchmarkBun_Update() (+14 more)

### Community 55 - "lruStmtCache"
Cohesion: 0.14
Nodes (9): boundedSyncMap, lruStmtCache, Element, EvictWriteStmt(), Mutex, TestEvictWriteStmt_NoPanic(), newBoundedSyncMap(), newLRUStmtCache() (+1 more)

### Community 56 - "BeginTrxFromContext"
Cohesion: 0.50
Nodes (4): BaseWrite, Context, CUDResponse, BeginTrxFromContext()

### Community 57 - ".Wrap"
Cohesion: 0.23
Nodes (7): StringMap, Store, Conn, Context, Duration, Group, Handler

### Community 58 - "response/response_test.go"
Cohesion: 0.05
Nodes (62): Buffer, ResponseWriter, NewJSON(), Context, Header, T, TestBadRequest(), TestChainedMethodCalls() (+54 more)

### Community 59 - "MockISQL"
Cohesion: 0.11
Nodes (4): CUDResponse, QueryOpts, Result, MockISQL

### Community 60 - "buildPivotEntries"
Cohesion: 0.20
Nodes (21): buildPivotEntries(), getBuilder(), getProcessedResult(), Builder, putBuilder(), putProcessedResult(), BenchmarkBuilderPool(), BenchmarkProcessedResultPool() (+13 more)

### Community 61 - "null/struct_test.go"
Cohesion: 0.11
Nodes (31): TestNewInt(), TestNewIntInvalid(), TestNullUint64(), IntFrom(), IntFromPtr(), NewInt(), NewUint64(), NullInt() (+23 more)

### Community 63 - "stubStore"
Cohesion: 0.25
Nodes (3): stubStore, Context, Duration

### Community 64 - "New"
Cohesion: 0.05
Nodes (61): TestGenerateAddOnQuery_WithNRTrx(), T, TestBeginTrxFromContextWithTransaction(), TestGetAppWithApp(), TestInitNewRelicNoAppNameFallback(), TestInitNewRelicWithAppNameFallback(), TestInitNewRelicWithEnvVars(), TestInitNewRelicWithNewAppError() (+53 more)

### Community 65 - "Helpers"
Cohesion: 0.04
Nodes (44): Auth Middleware, Banner Image Generator, Buckets Interface (`go/storage/definition.go:10`), Cloud Storage (GCS), Color Parsing, Context Logger, CSV Generator, Data Importer (+36 more)

### Community 66 - "struct_extended_test.go"
Cohesion: 0.12
Nodes (33): T, TestInt32From(), TestInt32FromPtr(), TestNewInt32(), TestNewInt64(), TestNewString(), TestNewStringInvalid(), TestNewUint() (+25 more)

### Community 67 - "ClientDeliveryManager"
Cohesion: 0.24
Nodes (4): RWMutex, NewClientDeliveryTracker(), ClientDeliveryManager, ClientDeliveryTracker

### Community 68 - "WithExtFile"
Cohesion: 0.25
Nodes (43): Example_importCSVWithProcess(), Example_importExcelXlsx(), WithCtx(), WithExtFile(), WithFile(), WithProcessFn(), WithSentNotifToSlack(), WithStructDestination() (+35 more)

### Community 69 - "Stmt"
Cohesion: 0.11
Nodes (7): fakeExecErrConn, fakeNoRowsConn, fakePrepareErrConn, fakeQueryRowConn, fakeRowsAffectedErrConn, Stmt, Tx

### Community 70 - "database/sql.go"
Cohesion: 0.14
Nodes (11): lruEntry, getBuilder(), Builder, Context, CUDResponse, SQL, QueryOpts, Stmt (+3 more)

### Community 71 - "mockStorage"
Cohesion: 0.24
Nodes (3): Context, File, mockStorage

### Community 72 - "NewBaseWrite"
Cohesion: 0.35
Nodes (15): BenchmarkPhastos_BulkInsert(), BenchmarkPhastos_Delete(), BenchmarkPhastos_Insert(), BenchmarkPhastos_Insert_Parallel(), BenchmarkPhastos_SelectByID(), BenchmarkPhastos_SelectByID_Parallel(), BenchmarkPhastos_SelectList(), BenchmarkPhastos_Update() (+7 more)

### Community 73 - "isql.go"
Cohesion: 0.15
Nodes (7): Controller, NewMockcommonDB(), NewMockFollower(), NewMockISQL(), NewMockMaster(), MockcommonDB, MockcommonDBMockRecorder

### Community 74 - "telegram_test.go"
Cohesion: 0.18
Nodes (21): New(), BotAPI, Server, T, newMockBot(), TestNewWithEmptyToken(), TestNewWithInvalidToken(), TestNewWithMockBot() (+13 more)

### Community 75 - "null/struct.go"
Cohesion: 0.20
Nodes (20): BoolFrom(), BoolFromPtr(), TestNewBool(), TestNewFloat32(), TestNullBool(), TestNullFloat32(), Float32From(), Float32FromPtr() (+12 more)

### Community 76 - "scheduler_coverage_test.go"
Cohesion: 0.22
Nodes (16): testWrapper, WithCronJob(), Context, T, TestAddScheduler(), TestAddScheduler_MultipleSchedulers(), TestStart_CORSEnvVars(), TestStart_NotifyServiceStatus() (+8 more)

### Community 77 - "controller.go"
Cohesion: 0.10
Nodes (30): Controller, ControllerConfig, ControllerImpl, ErrorResponseDoc, Handler, HeaderDoc, MessageResponse, MiddlewareInfo (+22 more)

### Community 78 - "Response"
Cohesion: 0.20
Nodes (4): Response, ResponseRecorder, ResponseWriter, NewResponseRecorder()

### Community 79 - "Time"
Cohesion: 0.11
Nodes (22): BaseModel, BeegoBenchUser, BenchOrder, BenchProduct, BenchUser, BunBenchUser, GormBenchOrder, GormBenchProduct (+14 more)

### Community 80 - "placeholders"
Cohesion: 0.24
Nodes (16): bulkPlaceholders(), createTables(), dropTables(), M, placeholder(), placeholders(), TestMain(), BenchmarkStdlib_BulkInsert() (+8 more)

### Community 81 - "monitor_test.go"
Cohesion: 0.13
Nodes (31): SetProviders(), T, TestCompositeProvider_GetLogLink_FirstNonEmpty(), TestCompositeProvider_GetTraceId_AllEmpty(), TestCompositeProvider_GetTraceId_FirstNonEmpty(), TestCompositeProvider_StartSpan_FanOut(), TestCompositeSpan_End_FanOut(), TestCompositeSpan_SetAttributes_FanOut() (+23 more)

### Community 82 - "Monitoring (Observability)"
Cohesion: 0.06
Nodes (32): Active Provider Lifecycle, API Integration, App Options, Auto-Init from Environment, Composite Provider (Both New Relic + OpenTelemetry), Configuration and Initialization, Configuration and Initialization, Default Config (+24 more)

### Community 83 - "UploadToTmp"
Cohesion: 0.22
Nodes (12): CheckFolder(), GetFolderAndFileName(), GetFolderNameWithoutTmp(), T, TestCheckFolder(), TestGetFilePath(), TestGetFolderAndFileName(), TestGetFolderNameWithoutTmp() (+4 more)

### Community 84 - "MockMaster"
Cohesion: 0.14
Nodes (3): Tx, TxOptions, MockMaster

### Community 85 - "WithTimeout"
Cohesion: 0.26
Nodes (19): actualRedisActionFn, FallbackFn, Handler, Options, RedisCfg, StreamData, StreamMessages, App (+11 more)

### Community 86 - ".ProcessData"
Cohesion: 0.24
Nodes (19): Context, NotifChannel(), NotifData(), NotifMsgType(), NotifTitle(), SendSlackNotification(), T, TestNotifChannel() (+11 more)

### Community 87 - "WithTimeZone"
Cohesion: 0.07
Nodes (49): Cron, contextKeyWrapper, Engine, Engines, option, Options, Response, testWrapper (+41 more)

### Community 88 - "NewTransaction"
Cohesion: 0.22
Nodes (14): Transaction, Tx, NewTransaction(), T, TestNewTransaction(), TestNewTransaction_NilDB(), TestTransaction_Begin_DelegatesToMaster(), TestTransaction_Begin_Error() (+6 more)

### Community 89 - "readPivot"
Cohesion: 0.13
Nodes (25): excel, File, excel, Value, File, readPivot(), readPivotChannel(), readPivotFromCSV() (+17 more)

### Community 90 - "excel"
Cohesion: 0.14
Nodes (7): excel, ExcelOptions, Excels, getColumnAlphabet(), Buffer, File, MergeCell

### Community 91 - "Request"
Cohesion: 0.28
Nodes (7): Request, FileHeader, File, ReleaseRequest(), getFileFromRequest(), File, Context

### Community 92 - "wraphandler_test.go"
Cohesion: 0.23
Nodes (16): generateUniqueRequestKey(), T, TestApp_WrapHandler_AsyncPath_ErrorResponse(), TestApp_WrapHandler_AsyncPath_Singleflight(), TestApp_WrapHandler_AsyncPath_Singleflight_ConcurrentPanic(), TestApp_WrapHandler_AsyncPath_Singleflight_HandlerNilReturn(), TestApp_WrapHandler_AsyncPath_Singleflight_HandlerPanic(), TestApp_WrapHandler_AsyncPath_SingleflightError() (+8 more)

### Community 93 - "fast_id_test.go"
Cohesion: 0.26
Nodes (14): GenerateFastID(), GenerateFastIDCounter(), GenerateFastIDCounterBytes(), PutFastIDCounterBytes(), T, TestGenerateFastID(), TestGenerateFastID_AllCharsValid(), TestGenerateFastID_Concurrent() (+6 more)

### Community 94 - "Banner"
Cohesion: 0.26
Nodes (7): Color, Banner, Banners, BgProperty, ImageLayer, Label, Image

### Community 95 - "slack"
Cohesion: 0.45
Nodes (3): Context, Response, slack

### Community 96 - "InitHandler"
Cohesion: 0.18
Nodes (9): WrittenResponseWriter, Handler, ResponseWriter, InitHandler(), ReleaseWrittenResponseWriter(), requestLogger(), T, TestInitHandler_RequestIdFromHeader() (+1 more)

### Community 97 - "WaitTermSig"
Cohesion: 0.21
Nodes (18): Config, Context, ServeHTTP(), ServeHTTPS(), WaitTermSig(), generateTempCert(), T, TestConfigStruct() (+10 more)

### Community 98 - "request_ctx_test.go"
Cohesion: 0.23
Nodes (13): mockHeaderer, requestIDKeyType, GetRequestID(), GetRequestIDFromHeader(), Context, T, TestGetRequestIDEmpty(), TestGetRequestIDFromHeader() (+5 more)

### Community 100 - "SSE Token Authentication Guide"
Cohesion: 0.06
Nodes (35): 1. Define Your Token Validator Function, 1. Token Generation, 1. Token Validation Flow, 2. Enable SSE with Token Validation, 2. Token Sources, 2. Token Validation, 3. Transport Security, 401 Unauthorized - Missing Token (+27 more)

### Community 101 - "stubPlatform"
Cohesion: 0.14
Nodes (4): Context, Handler, stubAction, stubPlatform

### Community 102 - "helper/struct_test.go"
Cohesion: 0.12
Nodes (31): ConstructColNameAndValue(), ConstructColNameAndValueBulk(), ConstructColNameAndValueForUpdate(), ConvertStructToMap(), generateBulk(), Context, CUDConstructData, Value (+23 more)

### Community 104 - "helper.go"
Cohesion: 0.23
Nodes (12): GetDateNowStringWithTimezone(), GetDateTimeNowStringWithFormat(), GetDateTimeNowStringWithTimezone(), GetTimeNowStringWithTimezone(), GetTimeNowWithTimezone(), ParseDate(), ParseToDateOnly(), ParseToTimeOnly() (+4 more)

### Community 105 - "Notifications"
Cohesion: 0.06
Nodes (31): Adding Handlers, Auto 500 Notification, Broadcasting, Core Types, Creating the App, Cron Scheduler, Enabling SSE, Event Type Dispatch Matrix (+23 more)

### Community 106 - "entity/slack/slack_test.go"
Cohesion: 0.25
Nodes (14): T, TestBaseIdNameStruct(), TestChannelCreateRequest(), TestChannelCreateRequestJSON(), TestChannelJSONRoundTrip(), TestChannelMemberResponse(), TestChannelStruct(), TestChannelWithTopicPurpose() (+6 more)

### Community 107 - "CryptoManager"
Cohesion: 0.21
Nodes (11): GeneratePublicKey(), NewCryptoManagerFromEnv(), T, TestEncrypt_AesErrorWithShortKey(), TestEncryptDecrypt(), TestGeneratePublicKey(), TestNewCryptoManager(), TestNewCryptoManagerFromEnv() (+3 more)

### Community 108 - "utils.go"
Cohesion: 0.12
Nodes (20): ValidationError, App, ResponseWriter, ArrContains(), decodeSchemaRequest(), filterFlags(), GenerateQueryComponenFromStruct(), getBodyFromJSON() (+12 more)

### Community 109 - "Basic Operations"
Cohesion: 0.07
Nodes (29): Basic Operations, Caches Interface, Connection, Connection Pool, Consumer Groups, Context Integration, Del, Fallback Flow (+21 more)

### Community 110 - "Test_nullValidator"
Cohesion: 0.40
Nodes (3): nullValidator(), T, Test_nullValidator()

### Community 111 - "GetBuffer"
Cohesion: 0.33
Nodes (12): Context, GetBuffer(), Buffer, PutBuffer(), T, TestBufferPoolWriteAfterGet(), TestGetBufferCompareBytes(), TestGetBufferConcurrentAccess() (+4 more)

### Community 112 - "File Structure"
Cohesion: 0.08
Nodes (24): File Structure, Global Constraints, Phastos README & Documentation Implementation Plan, Task 1: README.md — Landing Page, Task 2: docs/api.md — API Framework, Task 3: docs/database.md — SQL Query Builder, Task 4: docs/cache.md — Redis Cache, Task 5: docs/monitoring.md — Observability (+16 more)

### Community 113 - "FlattenJSONB"
Cohesion: 0.20
Nodes (23): buildExpandedHeaders(), expandArray(), expandJSONValue(), expandRecursive(), FlattenJSONB(), isJSONObjectOrArray(), sortedKeys(), T (+15 more)

### Community 114 - "echo_test.go"
Cohesion: 0.29
Nodes (10): BenchmarkEcho_JSON(), BenchmarkEcho_Middleware(), BenchmarkEcho_PathParam(), BenchmarkEcho_QueryParam(), echoMiddleware1(), echoMiddleware2(), echoMiddleware3(), B (+2 more)

### Community 115 - "serveFastHTTPs"
Cohesion: 0.27
Nodes (10): T, TestServeFastHTTPs_Basic(), TestServeFastHTTPs_GracefulShutdown(), TestServeFastHTTPs_InvalidPort(), TestServeFastHTTPs_PortAlreadyInUse(), TestServeFastHTTPs_PortBinding(), TestServeFastHTTPs_RequestHandler(), TestServeFastHTTPs_ResponseHeaders() (+2 more)

### Community 116 - "fiber_test.go"
Cohesion: 0.29
Nodes (11): App, BenchmarkFiber_JSON(), BenchmarkFiber_Middleware(), BenchmarkFiber_PathParam(), BenchmarkFiber_QueryParam(), fiberMiddleware1(), fiberMiddleware2(), fiberMiddleware3() (+3 more)

### Community 117 - "serveHTTPs"
Cohesion: 0.24
Nodes (16): T, itoa(), TestServeHTTPs_Basic(), TestServeHTTPs_InvalidPort(), TestServeHTTPs_PortInUse(), TestServeHTTPs_ShutdownOnSignal(), TestServeHTTPs_WithTestServer(), TestWaitTermSig_Basic() (+8 more)

### Community 118 - "beego_test.go"
Cohesion: 0.33
Nodes (10): BenchmarkBeego_BulkInsert(), BenchmarkBeego_Delete(), BenchmarkBeego_Insert(), BenchmarkBeego_Insert_Parallel(), BenchmarkBeego_SelectByID(), BenchmarkBeego_SelectByID_Parallel(), BenchmarkBeego_SelectList(), BenchmarkBeego_Update() (+2 more)

### Community 119 - "newStubConn"
Cohesion: 0.06
Nodes (114): stubConn, stubHandler, T, TestActualRedisActionFn_Type(), TestDel_WithNilContext(), TestFallbackFn_Type(), TestGet_WithNilTransaction(), TestHandler_Get() (+106 more)

### Community 120 - "Service"
Cohesion: 0.24
Nodes (5): FCMAttachment, FCMConfig, Service, Context, New()

### Community 121 - "PDF"
Cohesion: 0.11
Nodes (9): ConverterOptions, FileGenerator, PDF, PDFs, QR, QRs, Builder, PDFGenerator (+1 more)

### Community 122 - "storage/definition_test.go"
Cohesion: 0.16
Nodes (23): T, TestBucketsInterface(), TestConstants(), TestGoogleCloseRequiresClient(), TestGoogleDriveStruct(), TestGoogleInitResumableUploadsNoToken(), TestGoogleRollbackProcessCallsDeleteFile(), TestGoogleSetBucketName() (+15 more)

### Community 123 - "init_test.go"
Cohesion: 0.21
Nodes (23): ActivateSlack(), New(), T, TestActionInterface(), TestActivateSlack_NewSlackServiceError(), TestActivateSlackWithInvalidURL(), TestActivateSlackWithValidURL(), TestActivateTelegramWithInvalidToken() (+15 more)

### Community 124 - "Service"
Cohesion: 0.25
Nodes (3): BotAPI, Context, Service

### Community 126 - "generateUser"
Cohesion: 0.38
Nodes (10): generateUser(), BenchmarkSqlx_BulkInsert(), BenchmarkSqlx_Delete(), BenchmarkSqlx_Insert(), BenchmarkSqlx_Insert_Parallel(), BenchmarkSqlx_SelectByID(), BenchmarkSqlx_SelectByID_Parallel(), BenchmarkSqlx_SelectList() (+2 more)

### Community 127 - "xorm_test.go"
Cohesion: 0.38
Nodes (10): generateXormUser(), BenchmarkXorm_BulkInsert(), BenchmarkXorm_Delete(), BenchmarkXorm_Insert(), BenchmarkXorm_Insert_Parallel(), BenchmarkXorm_SelectByID(), BenchmarkXorm_SelectByID_Parallel(), BenchmarkXorm_SelectList() (+2 more)

### Community 128 - "Conn"
Cohesion: 0.18
Nodes (6): fakeConnectErrDriver, fakeExecErrDriver, fakePrepareErrDriver, fakeQueryRowDriver, fakeRowsAffectedErrDriver, Conn

### Community 129 - "string_case.go"
Cohesion: 0.29
Nodes (9): T, TestToCamelCase(), TestToLowerCamelCase(), TestToSnakeCase(), TestTrimDuplicatedSpace(), ToCamelCase(), ToLowerCamelCase(), ToSnakeCase() (+1 more)

### Community 130 - "string_generator.go"
Cohesion: 0.31
Nodes (9): GenerateRandomString(), GenerateRandomStringWithCharset(), GenerateUUID(), GenerateUUIDV4(), T, TestGenerateRandomString(), TestGenerateRandomStringWithCharset(), TestGenerateUUID() (+1 more)

### Community 131 - "template_test.go"
Cohesion: 0.39
Nodes (8): T, TestGetTemplate(), TestGetTemplateFS(), TestParseFileTemplate(), TestParseTemplate_ExecError(), TestParseTemplate_WithArgs(), TestParseTemplate_WithNilArgs(), TestParseTemplateFromPath_WithLocalFile()

### Community 132 - "compositeProvider"
Cohesion: 0.25
Nodes (3): Context, compositeProvider, compositeSpan

### Community 133 - "TestGoogleStorageExtended"
Cohesion: 0.22
Nodes (7): Response, T, TestGoogleStorageExtended(), Reader, RoundTripper, mockMultipartFile, rewriteTransport

### Community 134 - "Action"
Cohesion: 0.08
Nodes (12): stubActiveNotifPlatforms, stubNotifPlatforms, stubPlatforms, stubPlatforms, Context, Handler, Handler, Handler (+4 more)

### Community 135 - "New"
Cohesion: 0.18
Nodes (11): RequestError, New(), T, TestNew(), TestRequestError_AppendData(), TestRequestError_Chaining(), TestRequestError_Error(), TestRequestError_GetCode_Default() (+3 more)

### Community 136 - ".buildOperation"
Cohesion: 0.17
Nodes (15): routeRegistryEntry, autoErrorResponses(), autoTag(), generateSelectResponseSchema(), App, StructField, T, Type (+7 more)

### Community 137 - "errors.go"
Cohesion: 0.18
Nodes (19): constraintRegistryEntry, HttpError, ConflictError(), Forbidden(), MethodNotAllowed(), NotFound(), T, TestBadRequest() (+11 more)

### Community 138 - "actionFakeConn"
Cohesion: 0.22
Nodes (3): actionFakeConn, actionFakeErrorConn, Stmt

### Community 139 - "checkInitiateWhere"
Cohesion: 0.25
Nodes (8): checkInitiateWhere(), checkSortParam(), TableRequest, TestCheckInitiateWhere_NoConditions(), TestCheckInitiateWhere_SingleCondition(), TestCheckInitiateWhere_WithConditions(), TestCheckSortParam_WithOrderBy(), TestCheckSortParam_WithoutOrderBy()

### Community 140 - "entity_test.go"
Cohesion: 0.39
Nodes (8): T, TestCacheContextKey(), TestContextTypes(), TestJWTClaimData(), TestJWTClaimDataJSON(), TestJWTClaimDataWithNilData(), TestNotifDestinationContextKey(), TestNotifPlatformContextKey()

### Community 142 - "Global Constraints"
Cohesion: 0.10
Nodes (19): Auto-Binding Handler Implementation Plan, Global Constraints, Global Constraints, Self-Review Checklist, Task 10: Update timetraq go.mod + pilot migration on one controller, Task 1: Define `Handler2` type + handler detection, Task 1: `PathParamType` enum + typed `WithPath` parsing, Task 2: Change Route.Handler to `any` + add `Handler2` type (+11 more)

### Community 143 - "OpenAPI Docs — Auto-Generated API Documentation for Phastos"
Cohesion: 0.10
Nodes (19): AddController — merge metadata, Auto-Inject — ControllerImpl Tracking, ControllerImpl — rekam key otomatis, Data Model, Edge Cases, Endpoints, Middleware Metadata — Registration, MiddlewareInfo — metadata middleware di registry (+11 more)

### Community 144 - "HandlerFunc"
Cohesion: 0.31
Nodes (16): HandlerFunc, Handler, JWTAuth(), T, TestJWTAuth_BearerTokenPrefix(), TestJWTAuth_BearerTokenWithWhitespace(), TestJWTAuth_EmptySigningKey(), TestJWTAuth_ExpiredToken() (+8 more)

### Community 145 - "NewInt8"
Cohesion: 0.36
Nodes (8): TestNewInt8(), TestNullInt8(), Int8From(), Int8FromPtr(), NewInt8(), NullInt8(), TestInt8FromPtr(), Int8

### Community 146 - "NewErr"
Cohesion: 0.33
Nodes (13): ErrorOption, TestSentNotif_WithActiveSlackPlatform_Status500_WithCallerPath(), TestSentNotif_WithActiveSlackPlatform_Status500_WithData(), NewErr(), TestNewErr(), WithErrorCallerPath(), WithErrorCode(), WithErrorData() (+5 more)

### Community 147 - "SetNotif"
Cohesion: 0.37
Nodes (14): panicRecover(), T, TestPanicRecover_IntegrationWithApp(), TestPanicRecover_NilNotifContext(), TestPanicRecover_NoPanic(), TestPanicRecover_SyncPath_PanicHandledGracefully(), TestPanicRecover_WithPanic_InactivePlatform(), TestPanicRecover_WithPanic_MultiplePlatforms() (+6 more)

### Community 148 - "NewUint32"
Cohesion: 0.31
Nodes (9): TestNewUint32(), TestNullUint32(), TestUint32From(), TestUint32FromPtr(), NewUint32(), NullUint32(), Uint32From(), Uint32FromPtr() (+1 more)

### Community 149 - "NewUint8"
Cohesion: 0.31
Nodes (9): TestNewUint8(), TestNullUint8(), TestUint8From(), TestUint8FromPtr(), NewUint8(), NullUint8(), Uint8From(), Uint8FromPtr() (+1 more)

### Community 150 - "ISQL"
Cohesion: 0.18
Nodes (14): Base, baseAction, BaseWrites, deleteByIdCacheKey, HandlerCRUD, ReadHandler, ReadRepo, RepoCRUD (+6 more)

### Community 151 - "Design"
Cohesion: 0.12
Nodes (16): 10. Migration Strategy, 1. Handler Signature, 2. Route Annotations, 3. Binding Flow (Phastos Internal), 4. Context Package (`phastosctx`), 5. Response Wrapping (Phastos Internal), 6. Controller Example (Before/After), 7. Backward Compatibility (+8 more)

### Community 152 - "Route Group — Nested Group Routes for Phastos Controller"
Cohesion: 0.12
Nodes (15): `AddController`, Data Model, Edge Cases, Files Changed, `GetVersionedPath` — unchanged, Helper `NewGroup`, Middleware merge, Non-Goals (+7 more)

### Community 153 - "sqlTestFakeStmt"
Cohesion: 0.29
Nodes (4): sqlTestFakeStmt, Result, Rows, Value

### Community 154 - "FastJWTAuth"
Cohesion: 0.32
Nodes (15): T, TestFastJWTAuth_BearerPrefixWithSpace(), TestFastJWTAuth_EmptyToken(), TestFastJWTAuth_ExpiredToken(), TestFastJWTAuth_InvalidClaimsStruct(), TestFastJWTAuth_InvalidSignature(), TestFastJWTAuth_MissingAuthorizationHeader(), TestFastJWTAuth_MissingSigningKey() (+7 more)

### Community 155 - "NewFloat64"
Cohesion: 0.36
Nodes (8): TestNewFloat64(), TestNullFloat64(), Float64From(), Float64FromPtr(), NewFloat64(), NullFloat64(), TestFloat64FromPtr(), Float64

### Community 156 - "controller_test.go"
Cohesion: 0.19
Nodes (24): parsePathParamType(), parsePathParamTypes(), stripPathParamTypes(), T, TestControllerImpl_JoinMiddleware(), TestControllerImpl_UseMiddleware(), TestParsePathParamType_AllCases(), TestParsePathParamTypes_AllNumericTypes() (+16 more)

### Community 157 - "constants_test.go"
Cohesion: 0.48
Nodes (6): T, TestErrorConstants(), TestErrorConstantsAreNotNil(), TestErrorConstantsType(), TestErrorWrapping(), TestStringConstants()

### Community 158 - "CreateAsyncContext"
Cohesion: 0.33
Nodes (14): CreateAsyncContext(), Context, T, httptestRequest(), TestCreateAsyncContext_EmptyContext(), TestCreateAsyncContext_WithAllValues(), TestCreateAsyncContext_WithCache(), TestCreateAsyncContext_WithJWT() (+6 more)

### Community 159 - "GetNotif"
Cohesion: 0.30
Nodes (14): GetNotif(), GetNotifDestination(), Context, SetNotifDestination(), Context, T, TestGetNotif_NoNotifInContext(), TestGetNotif_WrongType() (+6 more)

### Community 160 - ".Register"
Cohesion: 0.40
Nodes (5): mockControllers, Controller, init(), init(), init()

### Community 162 - "binding_extended_test.go"
Cohesion: 0.15
Nodes (23): testStructValidation, Bind(), decodeSchemaRequest(), T, TestBindDeleteRequestNoBody(), TestBindGETRequest(), TestBindInvalidContentType(), TestBindJSONPostRequest() (+15 more)

### Community 163 - "cache_test.go"
Cohesion: 0.25
Nodes (12): Caches, T, TestDefaultPrefixKey(), TestGetCacheFromContext_WhenAbsent(), TestGetCacheFromContext_WhenPresent(), TestGetCacheFromContext_WhenWrongType(), TestRedisCfgDefaults(), TestStoreWrapToContext() (+4 more)

### Community 164 - "TestRemove"
Cohesion: 0.40
Nodes (4): SliceContains(), T, TestRemove(), TestSliceContains()

### Community 165 - "processing_test.go"
Cohesion: 0.17
Nodes (19): File, Image, Load(), T, TestLoadFromHTTPURL(), TestLoadFromHTTPURLInvalidImage(), TestLoadFromHTTPURLServerDown(), TestLoadInvalidFileContent() (+11 more)

### Community 166 - "StaticAuth"
Cohesion: 0.29
Nodes (12): Handler, ResponseWriter, StaticAuth(), T, TestStaticAuth_CorrectToken(), TestStaticAuth_EmptyEnvSecret(), TestStaticAuth_MissingToken(), TestStaticAuth_TraceId() (+4 more)

### Community 167 - "server/definition.go"
Cohesion: 0.33
Nodes (5): Context, Handler, Config, GRPC, HTTP

### Community 168 - "stubResponseWriter"
Cohesion: 0.40
Nodes (3): Builder, Header, stubResponseWriter

### Community 169 - "Channel"
Cohesion: 0.16
Nodes (13): T, AuthedUser, BaseIdName, Channel, ChannelCreateRequest, ChannelMemberResponse, Latest, OauthAccess (+5 more)

### Community 170 - "monitor.go"
Cohesion: 0.23
Nodes (8): GetLogLink(), GetTraceId(), Context, Init(), StartSpan(), noopProvider, noopSpan, Span

### Community 171 - "PanicHandler"
Cohesion: 0.27
Nodes (11): Handler, ResponseWriter, MethodNotAllowedHandler(), PanicHandler(), RouteNotFoundHandler(), T, TestMethodNotAllowedHandler(), TestPanicHandler_CatchesPanic() (+3 more)

### Community 173 - "mockControllerWithImpl"
Cohesion: 0.67
Nodes (3): mockControllerWithImpl, ControllerConfig, ControllerImpl

### Community 174 - "middlewares_extended_test.go"
Cohesion: 0.28
Nodes (12): T, TestDefaultKeyExtractor(), TestNewRateLimiterAllowsRequest(), TestNewRateLimiterDefaults(), TestNewRateLimiterSkipPath(), TestStaticAuthNoHeader(), TestStaticAuthWrongToken(), TestUnauthorizedInvalidToken() (+4 more)

### Community 175 - "server/definition_test.go"
Cohesion: 0.28
Nodes (12): T, TestConfigDefaults(), TestConfigWithHandler(), TestConfigWithValues(), TestGRPCInterface(), TestHTTPInterface(), TestServeHTTP(), TestServeHTTPSCallsServeHTTPs() (+4 more)

### Community 176 - "entity/context.go"
Cohesion: 0.50
Nodes (3): CacheContext, NotifDestinationContext, NotifPlatformContext

### Community 178 - "MessageBuffer"
Cohesion: 0.22
Nodes (3): Duration, BufferedMessage, MessageBuffer

### Community 183 - "Global Constraints"
Cohesion: 0.20
Nodes (9): Global Constraints, Route Group Implementation Plan, Task 1: Add SubRoutes + NewGroup to Route (controller.go), Task 2: Add SubRoutes + NewGroup to FastRoute (fasthttp_app.go), Task 3: Recursive registerRoutes for chi path (app.go), Task 4: Recursive registerRoutes for fasthttp path (fasthttp_app.go), Task 5: Tests for chi route groups (app_extended_test.go), Task 6: Tests for fasthttp route groups (fasthttp_coverage_test.go) (+1 more)

### Community 186 - "GetJWT"
Cohesion: 0.24
Nodes (8): JwtContext, JWTClaimData, GetJWT(), Context, SetJWT(), T, TestSetAndGetJWT(), RegisteredClaims

### Community 188 - "Options Functions"
Cohesion: 0.22
Nodes (8): api — `api.Options`, cache — `cache.Options`, Configuration Reference, cron — `cron.Options`, Environment Variables, log — `log.LoggerOption`, monitoring, Options Functions

### Community 192 - "Global Constraints"
Cohesion: 0.22
Nodes (8): Global Constraints, OpenAPI Docs Implementation Plan, Task 1: RouteDoc data model + RouteOption helpers + MiddlewareInfo registry, Task 2: ControllerImpl tracking + AddController auto-inject, Task 3: OpenAPI spec generator, Task 4: Serve /docs + /docs/openapi.json, Task 5: Tests, Task 6: Final verification

### Community 194 - "NewInt16"
Cohesion: 0.31
Nodes (9): TestInt16From(), TestInt16FromPtr(), TestNewInt16(), TestNullInt16(), Int16From(), Int16FromPtr(), NewInt16(), NullInt16() (+1 more)

### Community 195 - "checkKeyword"
Cohesion: 0.25
Nodes (8): checkKeyword(), generateParamArgsForLike(), TestCheckKeyword_KeywordWithoutSearchCols(), TestCheckKeyword_NoKeyword(), TestCheckKeyword_WithKeywordAndSearchCols(), TestCheckKeyword_WithKeywordAndSearchCols_WithInitiateWhere(), TestGenerateParamArgsForLike(), TestGenerateParamArgsForLike_Empty()

### Community 198 - "SetProvider"
Cohesion: 0.25
Nodes (8): ActiveProvider(), SetProvider(), TestActiveProvider(), TestInitNewRelicOnly_ReturnsProvider(), TestSetProvider(), TestSetProvider_Nil(), TestStartSpan_CallsProvider(), InitNewRelicOnly()

### Community 199 - "stubAction"
Cohesion: 0.25
Nodes (3): Context, TestStubActionSetTraceId(), stubAction

### Community 200 - "notifications/init.go"
Cohesion: 0.38
Nodes (5): App, ActivateFirebase(), ActivateTelegram(), Options, Platforms

### Community 201 - "HandlerV2 on Version-Group Sub-Router: Bug Fixes"
Cohesion: 0.29
Nodes (6): Bug 1: `stripPathParamTypes` corrupted path param patterns, Bug 2: Chi wildcard param `*` in Mounted sub-router, HandlerV2 on Version-Group Sub-Router: Bug Fixes, Problem, Root Cause (Two Bugs), Summary

### Community 202 - "Phastos"
Cohesion: 0.33
Nodes (6): Project Changelog, Auto Tag CI Workflow, Golangci-lint Configuration, New Relic Integration, OpenTelemetry Integration, Phastos

### Community 203 - "PhastosFastHttpNative"
Cohesion: 0.40
Nodes (6): GenerateFastID, Header-Based Request ID, PhastosChi, PhastosFastHttpNative, Profile-Guided Optimizations, Sync Handler Path Optimization

### Community 204 - "Fix Singleflight NPE in wrapHandler"
Cohesion: 0.40
Nodes (4): File Structure, Fix Singleflight NPE in wrapHandler, Global Constraints, Task 1: Fix goroutine in wrapHandler — recover + nil-safety

### Community 205 - "GenerateJWTToken"
Cohesion: 0.40
Nodes (3): GenerateJWTToken(), T, TestGenerateJWTToken()

### Community 207 - "NewDrive"
Cohesion: 0.50
Nodes (4): Context, NewDrive(), Service, googleDrive

### Community 209 - "run.sh"
Cohesion: 0.50
Nodes (3): BENCHMARK_DB_DSN, BENCHMARK_DB_TYPE, run.sh script

### Community 210 - "API Benchmarks"
Cohesion: 1.00
Nodes (3): API Benchmarks, DB/ORM Benchmarks, Benchmark Overview

### Community 212 - "github.com/kodekoding/phastos/v2"
Cohesion: 0.67
Nodes (3): api, benchmarkdb, github.com/kodekoding/phastos/v2

## Knowledge Gaps
- **380 isolated node(s):** `api`, `JSONResponse`, `benchmarkdb`, `run.sh script`, `BENCHMARK_DB_TYPE` (+375 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **32 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `App` connect `App` to `otel.go`, `Hub`, `.buildOperation`, `controller.go`, `NewErr`, `ISQL`, `WithTimeZone`, `NewApp`?**
  _High betweenness centrality (0.107) - this node is a cross-community bridge._
- **Why does `Transactions` connect `App` to `NewTransaction`, `readPivot`, `WithExtFile`?**
  _High betweenness centrality (0.105) - this node is a cross-community bridge._
- **Why does `HandlerFunc` connect `HandlerFunc` to `otel.go`, `TestGoogleStorageExtended`, `App`, `runNetHTTPBenchmark`, `helper_coverage_test.go`, `NewRateLimiter`, `WithAPITimeout`, `Platform`, `slack/api_test.go`, `NewApp`, `cache_test.go`, `processing_test.go`, `StaticAuth`, `PanicHandler`, `middlewares_extended_test.go`, `server/definition_test.go`, `.Wrap`, `New`, `telegram_test.go`, `.ProcessData`, `WithTimeZone`, `InitHandler`, `WaitTermSig`, `echo_test.go`, `serveHTTPs`, `newStubConn`, `init_test.go`?**
  _High betweenness centrality (0.098) - this node is a cross-community bridge._
- **Are the 114 inferred relationships involving `NewApp()` (e.g. with `setupPhastosChiApp()` and `TestApp_AddController_InjectsMiddlewares()`) actually correct?**
  _`NewApp()` has 114 INFERRED edges - model-reasoned connections that need verification._
- **Are the 107 inferred relationships involving `WithTimezone()` (e.g. with `TestApp_AddController_InjectsMiddlewares()` and `TestApp_flushPendingMiddlewares_WithGlobalMiddlewares()`) actually correct?**
  _`WithTimezone()` has 107 INFERRED edges - model-reasoned connections that need verification._
- **What connects `api`, `JSONResponse`, `benchmarkdb` to the rest of the system?**
  _380 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `sql_deep_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.052606177606177605 - nodes in this community are weakly interconnected._