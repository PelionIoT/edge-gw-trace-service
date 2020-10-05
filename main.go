package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"
	"github.com/armPelionEdge/muuid-go"
	"edge-gw-trace-service/log"
	"edge-gw-trace-service/routes"
	"edge-gw-trace-service/storage"
	"edge-gw-trace-service/tracing"
	"edge-gw-trace-service/tokens"
	"edge-gw-trace-service/services"

	"github.com/armPelionEdge/edge-gw-services-go/middleware"
	"github.com/armPelionEdge/edge-gw-services-go/middleware/access_tokens"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gorilla/mux"
	"github.com/dgrijalva/jwt-go"
)

func main() {
	// Parse the url from the command line
	var esURL string
	var esSearchAlias string
	var esActiveAlias string
	var loggingLevel string
	var uuidNetworkInterface string
	var jwtKey string
	var deviceDirectoryURLStr string
	var jwtIssuer string
	var jwtExpSeconds int64
	var jwtSigningKeyFile string
	flag.StringVar(&esURL, "esURL", "", "The host address for elastic search service")
	flag.StringVar(&esSearchAlias, "esSearchAlias", "", "The search alias name for the elastic search service")
	flag.StringVar(&esActiveAlias, "esActiveAlias", "", "The active alias name for the elastic search service")
	flag.StringVar(&loggingLevel, "loggingLevel", "debug", "The level of logging desired")
	flag.StringVar(&uuidNetworkInterface, "uuidNetworkInterface", "eth0", "The network interface to be used for uuid generation")
	flag.StringVar(&jwtKey, "jwtKey", "", "Public key used for decoding token")
	flag.StringVar(&deviceDirectoryURLStr, "deviceDirectoryURL", "", "Root URL of the device directory service")
	flag.StringVar(&jwtIssuer, "jwtIssuer", "gateway-trace", "Issuer field for JWT tokens")
	flag.Int64Var(&jwtExpSeconds, "jwtExpiration", 60, "JWT expiration time in seconds")
	flag.StringVar(&jwtSigningKeyFile, "jwtSigningKey", "", "Private key used for JWT signing")
	flag.Parse()

	if esURL == "" {
		fmt.Fprintf(os.Stderr, "Argument \"esURL\" is required.\n")
		os.Exit(1)
	}

	if esSearchAlias == "" {
		fmt.Fprintf(os.Stderr, "Argument \"esSearchAlias\" is required.\n")
		os.Exit(1)
	}

	if esActiveAlias == "" {
		fmt.Fprintf(os.Stderr, "Argument \"esActiveAlias\" is required.\n")
		os.Exit(1)
	}

	if jwtKey == "" {
		fmt.Fprintf(os.Stderr, "Argument \"jwtKey\" is required.\n")
		os.Exit(1)
	}

	if deviceDirectoryURLStr == "" {
		fmt.Fprintf(os.Stderr, "Argument \"deviceDirectoryURL\" is required.\n")
		os.Exit(1)
	}

	if jwtSigningKeyFile == "" {
		fmt.Fprintf(os.Stderr, "Argument \"jwtSigningKey\" is required.\n")
		os.Exit(1)
	}

	jwtKeyPEM, err := ioutil.ReadFile(jwtKey)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read file specified by jwt-key file path at %s: %v\n", jwtKey, err)

		os.Exit(1)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(jwtKeyPEM)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse file specified by jwt-key at %s: %v\n", jwtKey, err)

		os.Exit(1)
	}

	deviceDirectoryURL, err := url.Parse(deviceDirectoryURLStr)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\"deviceDirectoryURL\" could not be parsed: %s\n", err.Error())

		os.Exit(1)
	}

	jwtSigningKeyPEM, err := ioutil.ReadFile(jwtSigningKeyFile)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read JWT signing key file: %s\n", jwtSigningKeyFile)
		os.Exit(1)
	}

	jwtSigningKey, err := jwt.ParseRSAPrivateKeyFromPEM(jwtSigningKeyPEM)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse JWT signing key PEM: %s\n", jwtSigningKeyFile)
		os.Exit(1)
	}

	// Set up zap logging component
	atom := zap.NewAtomicLevel()
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.Lock(os.Stdout),
		atom,
	), zap.AddCaller())

	// Set Logging Level
	atom.SetLevel(log.ZapLogLevel(loggingLevel))

	// Initialize an muuid generator
	var muuidGeneratorBuilder muuid.MUUIDGeneratorBuilder = muuid.MUUIDGeneratorBuilder{
		NetworkInterface: uuidNetworkInterface,
		InstanceId: 1,
	}

	uuidGenerator, err := muuidGeneratorBuilder.Build()

	// Generate a jwt token factory
	tokenFactory := tokens.JWTTokenFactory{
		Issuer:     jwtIssuer,
		TokenExp:   time.Duration(jwtExpSeconds) * time.Second,
		SigningKey: jwtSigningKey,
	}

	// Init access token decoder
	armAccessTokenGetter := &access_tokens.ArmAccessTokenGetterImpl{}
	armAccessTokenDecoder := &access_tokens.ArmAccessTokenDecoderImpl{
		PublicKey: publicKey,
	}

	// Initialize an instance of the ESTraceStore
	esTraceStore, err := storage.NewESTraceStore(logger.With(zap.String("component", "storage.ESTraceStore")), esURL, esSearchAlias, esActiveAlias)

	if err != nil {
		logger.Error("main(): Failed to connect to the ElasticSearch server.", zap.String("esURL", esURL), zap.Error(err))
		os.Exit(1)
	}

	router := mux.NewRouter()

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
	}
	logger.Debug("main(): Setting up web server.. ")

	// Initialize an instance of the TraceEndpoint and initialize TraceStore with the instance of ESTraceStore
	TraceEndpoint := routes.TraceEndpoint {
		TraceStore            : esTraceStore,
		AccessTokenMiddleware : middleware.ArmAccessTokenMiddleware(armAccessTokenGetter, armAccessTokenDecoder),
		UUIDGenerator         : &uuidGenerator,
		DeviceDirectory       : &services.DeviceDirectoryImpl {
			Client : services.Client {
				Client    : http.DefaultClient,
				JWTFactory: &tokenFactory,
				Logger    : logger.With(zap.String("component", "device-directory-client")),
			},
			DeviceDirectoryServiceURL : deviceDirectoryURL,
		},
		Logger                : logger.With(zap.String("component", "routes.TraceEndpoint")),
	}

	// Attach the router to the TraceEndpoint
	TraceEndpoint.Attach(router)

	// Start opentracing
	closer, err := tracing.Start(logger.With(zap.String("component", "opentracing")))

	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not start tracing: %s", err)

		os.Exit(1)
	}

	defer closer.Close()

	// Gracefully shutdown the server
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("main(): Server Setup Error", zap.Error(err))
		}
	}()

	logger.Debug("main(): Successfully set up the server")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	srv.Shutdown(ctx)
	logger.Debug("main(): Server Shutting down")
	os.Exit(0)
}
