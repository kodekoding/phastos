module github.com/kodekoding/phastos/v2

go 1.23.0

require (
	cloud.google.com/go/storage v1.49.0
	github.com/SebastiaanKlippert/go-wkhtmltopdf v1.7.2
	github.com/ashwanthkumar/slack-go-webhook v0.0.0-20200209025033-430dd4e66960
	github.com/disintegration/imaging v1.6.2
	github.com/extrame/xls v0.0.1
	github.com/fogleman/gg v1.3.0
	github.com/go-chi/chi v1.5.4
	github.com/go-chi/chi/v5 v5.2.0
	github.com/go-chi/cors v1.2.0
	github.com/go-playground/validator v9.31.0+incompatible
	github.com/go-resty/resty/v2 v2.7.0
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/golang-jwt/jwt/v4 v4.5.1
	github.com/gomodule/redigo v1.8.9
	github.com/google/uuid v1.6.0
	github.com/gorilla/schema v1.4.1
	github.com/iancoleman/strcase v0.3.0
	github.com/jmoiron/sqlx v1.3.4
	github.com/keighl/mandrill v0.0.0-20170605120353-1775dd4b3b41
	github.com/lib/pq v1.10.2
	github.com/mauri870/gcsfs v0.0.0-20220203135357-0da01ba4e96d
	github.com/newrelic/go-agent/v3 v3.35.1
	github.com/newrelic/go-agent/v3/integrations/logcontext-v2/zerologWriter v1.0.2
	github.com/newrelic/go-agent/v3/integrations/nrmysql v1.2.2
	github.com/newrelic/go-agent/v3/integrations/nrpq v1.1.1
	github.com/pkg/errors v0.9.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/rs/zerolog v1.33.0
	github.com/satori/go.uuid v1.2.0
	github.com/slack-go/slack v0.12.1
	github.com/stretchr/testify v1.10.0
	github.com/unrolled/secure v1.13.0
	github.com/volatiletech/null v8.0.0+incompatible
	github.com/xuri/excelize/v2 v2.7.0
	github.com/yeqown/go-qrcode v1.5.10
	golang.org/x/sync v0.10.0
	google.golang.org/api v0.214.0
	gorm.io/driver/mysql v1.3.4
)

require (
	cel.dev/expr v0.16.2 // indirect
	cloud.google.com/go v0.116.0 // indirect
	cloud.google.com/go/auth v0.13.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.6 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	cloud.google.com/go/iam v1.2.2 // indirect
	cloud.google.com/go/monitoring v1.21.2 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.25.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.48.1 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.48.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20240905190251-b4127c9b8d78 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/elazarl/goproxy v0.0.0-20231117061959-7cc037d33fb5 // indirect
	github.com/envoyproxy/go-control-plane v0.13.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.1.0 // indirect
	github.com/extrame/ole2 v0.0.0-20160812065207-d69429661ad7 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/friendsofgo/errors v0.9.2 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-playground/locales v0.14.0 // indirect
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.14.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.4 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrwriter v1.0.0 // indirect
	github.com/parnurzeal/gorequest v0.2.16 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.3 // indirect
	github.com/smartystreets/goconvey v1.8.1 // indirect
	github.com/volatiletech/inflect v0.0.1 // indirect
	github.com/volatiletech/sqlboiler v3.7.1+incompatible // indirect
	github.com/xuri/efp v0.0.0-20220603152613-6918739fd470 // indirect
	github.com/xuri/nfp v0.0.0-20220409054826-5e722a1d9e22 // indirect
	github.com/yeqown/reedsolomon v1.0.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.31.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.54.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.54.0 // indirect
	go.opentelemetry.io/otel v1.33.0 // indirect
	go.opentelemetry.io/otel/metric v1.33.0 // indirect
	go.opentelemetry.io/otel/sdk v1.33.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.33.0 // indirect
	go.opentelemetry.io/otel/trace v1.33.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/image v0.0.0-20220902085622-e7cb96979f69 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/oauth2 v0.24.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/genproto v0.0.0-20241118233622-e639e219e697 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241118233622-e639e219e697 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241209162323-e6fa225c2576 // indirect
	google.golang.org/grpc v1.69.2 // indirect
	google.golang.org/protobuf v1.35.2 // indirect
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorm.io/gorm v1.23.4 // indirect
	moul.io/http2curl v1.0.0 // indirect
)
