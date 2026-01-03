# https://mcr.microsoft.com/en-us/artifact/mar/dotnet/sdk/tags
FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /dapr

# Copy everything
COPY . ./
# Restore as distinct layers
RUN dotnet restore
# Build and publish a release
RUN dotnet publish -c Release -o out

# https://mcr.microsoft.com/en-us/artifact/mar/dotnet/runtime/tags
FROM mcr.microsoft.com/dotnet/aspnet:10.0 AS final
WORKDIR /dapr
COPY --from=build /dapr/out .
ENTRYPOINT ["dotnet", "order-service.dll"]