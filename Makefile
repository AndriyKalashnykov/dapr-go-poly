
export KO_DOCKER_REPO=docker.io/andriykalashnykov

test:
	@cd ./basket-service; go test ./...
	@cd ./onboarding; go test ./...

build:
	@cd basket-service && go mod download && go build -o main main.go
	@cd basket-service && go mod download && go build -o main main.go
	@cd order-service && dotnet build order-service.csproj && cd ..
	@cd product-service && dotnet build product-service.csproj && cd ..

update:
	@cd basket-service; go get -u ./...; go mod tidy
	@cd onboarding; go get -u ./...; go mod tidy
	@cd order-service && dotnet list package --outdated | grep -o '> \S*' | grep '[^> ]*' -o | xargs --no-run-if-empty -L 1 dotnet add package
	@cd product-service && dotnet list package --outdated | grep -o '> \S*' | grep '[^> ]*' -o | xargs --no-run-if-empty -L 1 dotnet add package

dapr-run:
	@cd order-service && dapr run --config ../.dapr/config.yaml --app-id product-service --app-port 8080 --placement-host-address host.docker.internal:50006 --dapr-http-port 3500

compose-up:
	docker compose up --build

compose-down:
	docker compose down
