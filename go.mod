module github.com/topfreegames/maestro

go 1.17

require (
	github.com/DataDog/datadog-go/v5 v5.3.0
	github.com/GuiaBolso/darwin v0.0.0-20170210191649-86919dfcf808
	github.com/asaskevich/govalidator v0.0.0-20161001163130-7b3beb6df3c4
	github.com/bsm/redis-lock v6.0.0+incompatible
	github.com/btcsuite/btcutil v0.0.0-20180706230648-ab6388e0c60a
	github.com/getlantern/deepcopy v0.0.0-20140913144530-b923171e8640
	github.com/getsentry/raven-go v0.0.0-20170310193735-b68337dbf03e
	github.com/go-pg/pg v6.13.2+incompatible
	github.com/go-redis/redis v6.12.0+incompatible
	github.com/golang/mock v1.6.0
	github.com/google/uuid v0.0.0-20161128191214-064e2069ce9c
	github.com/gorilla/mux v1.6.1
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/lib/pq v0.0.0-20170324204654-2704adc878c2
	github.com/mitchellh/go-homedir v0.0.0-20161203194507-b8bc1bf76747
	github.com/mmcloughlin/professor v0.0.0-20170922221822-6b97112ab8b3
	github.com/newrelic/go-agent v1.9.0
	github.com/onsi/ginkgo v1.3.2-0.20170409210154-f40a49d81e5c
	github.com/onsi/gomega v1.4.1
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pmylund/go-cache v2.0.0+incompatible
	github.com/rs/cors v0.0.0-20170727213201-7af7a1e09ba3
	github.com/satori/go.uuid v1.1.0
	github.com/sergi/go-diff v1.0.0
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v0.0.0-20170408144537-5deb57bbca49
	github.com/spf13/viper v1.1.0
	github.com/topfreegames/extensions v6.6.2+incompatible
	github.com/topfreegames/go-extensions-k8s-client-go v1.1.1-0.20190906200922-71e4464e67cb
	github.com/topfreegames/protos v1.8.0
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	google.golang.org/grpc v1.14.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.0.0-20190805182141-5e2f71e44c7f
	k8s.io/apimachinery v0.0.0-20190629003722-e20a3a656cff
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/metrics v0.0.0-20190805184908-cf97d17242fb
)

require (
	cloud.google.com/go v0.34.0 // indirect
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.1.0 // indirect
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/certifi/gocertifi v0.0.0-20170417193930-a9c833d2837d // indirect
	github.com/cznic/ql v1.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/google/gofuzz v1.0.0 // indirect
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gorilla/context v0.0.0-20160226214623-1ea25387ff6f // indirect
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v0.0.0-20170217164738-630949a3c5fa // indirect
	github.com/imdario/mergo v0.0.0-20160216103600-3e95a51e0639 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jinzhu/inflection v0.0.0-20170102125226-1c35d901db3d // indirect
	github.com/json-iterator/go v1.1.6 // indirect
	github.com/magiconair/properties v1.7.2 // indirect
	github.com/mattn/goveralls v0.0.12 // indirect
	github.com/mitchellh/mapstructure v0.0.0-20170307201123-53818660ed49 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pelletier/go-toml v1.2.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/afero v0.0.0-20170217164146-9be650865eab // indirect
	github.com/spf13/cast v1.0.0 // indirect
	github.com/spf13/jwalterweatherman v0.0.0-20170109133355-fa7ca7e836cf // indirect
	github.com/spf13/pflag v0.0.0-20170412152249-e453343e6260 // indirect
	github.com/topfreegames/go-extensions-http v1.0.0 // indirect
	github.com/topfreegames/go-extensions-tracing v1.1.0 // indirect
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	github.com/wadey/gocovmerge v0.0.0-20160331181800-b5bfa59ec0ad // indirect
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/term v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	golang.org/x/tools v0.8.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.4.0 // indirect
	google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8 // indirect
	google.golang.org/protobuf v1.23.0 // indirect
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0 // indirect
	k8s.io/klog v0.4.0 // indirect
	k8s.io/kube-openapi v0.0.0-20180731170545-e3762e86a74c // indirect
	sigs.k8s.io/yaml v1.1.1-0.20190704183835-4cd0c284b15f // indirect
)

replace github.com/golang/mock => github.com/golang/mock v1.0.0
