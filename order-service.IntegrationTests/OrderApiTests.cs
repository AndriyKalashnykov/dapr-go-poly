using System.Net;
using System.Net.Http.Json;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Testcontainers.PostgreSql;
using TUnit.Assertions;
using TUnit.Assertions.Extensions;
using TUnit.Core;
using TUnit.Core.Interfaces;

namespace order_service.IntegrationTests;

public sealed class OrderApiFixture : IAsyncInitializer, IAsyncDisposable
{
    public PostgreSqlContainer Postgres { get; } = new PostgreSqlBuilder("postgres:17-alpine")
        .WithDatabase("order")
        .WithUsername("postgres")
        .WithPassword("test")
        .Build();

    public WebApplicationFactory<Program> Factory { get; private set; } = null!;

    public async Task InitializeAsync()
    {
        await Postgres.StartAsync();

        Environment.SetEnvironmentVariable("POSTGRES_HOST", Postgres.Hostname);
        Environment.SetEnvironmentVariable("POSTGRES_PORT", Postgres.GetMappedPublicPort(5432).ToString());
        Environment.SetEnvironmentVariable("POSTGRES_DATABASE", "order");
        Environment.SetEnvironmentVariable("POSTGRES_USERNAME", "postgres");
        Environment.SetEnvironmentVariable("POSTGRES_PASSWORD", "test");
        Environment.SetEnvironmentVariable("PRODUCT_SERVICE_BASE_URL", "http://product-service.invalid");

        Factory = new WebApplicationFactory<Program>()
            .WithWebHostBuilder(b => b.UseEnvironment("Test"));
    }

    public async ValueTask DisposeAsync()
    {
        await Factory.DisposeAsync();
        await Postgres.DisposeAsync();
    }
}

[ClassDataSource<OrderApiFixture>(Shared = SharedType.PerClass)]
[Category("Integration")]
public sealed class OrderApiTests(OrderApiFixture fixture)
{
    [Test]
    public async Task Get_ReturnsEmptyList_OnFreshDatabase()
    {
        using var client = fixture.Factory.CreateClient();
        var resp = await client.GetAsync("/api/orders");
        await Assert.That(resp.StatusCode).IsEqualTo(HttpStatusCode.OK);
        var orders = await resp.Content.ReadFromJsonAsync<List<OrderDto>>();
        await Assert.That(orders).IsNotNull();
        await Assert.That(orders!.Count).IsEqualTo(0);
    }

    public sealed record OrderDto(Guid Id, Guid ProductId, uint Quantity, decimal Price);
}
