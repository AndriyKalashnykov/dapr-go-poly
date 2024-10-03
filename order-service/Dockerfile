FROM mcr.microsoft.com/dotnet/sdk:9.0-alpine AS build-env
WORKDIR /dapr

# Copy everything
COPY . ./
# Restore as distinct layers
RUN dotnet restore
# Build and publish a release
RUN dotnet publish -c Release -o out

# Build runtime image
FROM mcr.microsoft.com/dotnet/aspnet:9.0-alpine
WORKDIR /dapr
COPY --from=build-env /dapr/out .
ENTRYPOINT ["dotnet", "order-service.dll"]