FROM mcr.microsoft.com/dotnet/sdk:9.0.101 AS build
WORKDIR /dapr

# Copy everything
COPY . ./
# Restore as distinct layers
RUN dotnet restore
# Build and publish a release
RUN dotnet publish -c Release -o out

# Build runtime image
FROM mcr.microsoft.com/dotnet/runtime:8.0.11-cbl-mariner2.0
WORKDIR /dapr
COPY --from=build /dapr/out .
ENTRYPOINT ["dotnet", "order-service.dll"]