using System.Net;
using System.Net.Http.Json;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Testcontainers.PostgreSql;
using Xunit;

namespace order_service.IntegrationTests;

[Trait("Category", "Integration")]
public sealed class OrderApiTests : IClassFixture<OrderApiTests.Fixture>
{
    private readonly Fixture _fixture;

    public OrderApiTests(Fixture fixture) => _fixture = fixture;

    [Fact]
    public async Task Get_ReturnsEmptyList_OnFreshDatabase()
    {
        var resp = await _fixture.Client.GetAsync("/api/orders");
        Assert.Equal(HttpStatusCode.OK, resp.StatusCode);
        var orders = await resp.Content.ReadFromJsonAsync<List<OrderDto>>();
        Assert.NotNull(orders);
        Assert.Empty(orders!);
    }

    public sealed record OrderDto(Guid Id, Guid ProductId, uint Quantity, decimal Price);

    public sealed class Fixture : IAsyncLifetime
    {
        private readonly PostgreSqlContainer _postgres = new PostgreSqlBuilder()
            .WithImage("postgres:17-alpine")
            .WithDatabase("order")
            .WithUsername("postgres")
            .WithPassword("test")
            .Build();

        public WebApplicationFactory<Program> Factory { get; private set; } = null!;
        public HttpClient Client { get; private set; } = null!;

        public async Task InitializeAsync()
        {
            await _postgres.StartAsync();

            Environment.SetEnvironmentVariable("POSTGRES_HOST", _postgres.Hostname);
            Environment.SetEnvironmentVariable("POSTGRES_PORT", _postgres.GetMappedPublicPort(5432).ToString());
            Environment.SetEnvironmentVariable("POSTGRES_DATABASE", "order");
            Environment.SetEnvironmentVariable("POSTGRES_USERNAME", "postgres");
            Environment.SetEnvironmentVariable("POSTGRES_PASSWORD", "test");
            Environment.SetEnvironmentVariable("PRODUCT_SERVICE_BASE_URL", "http://product-service.invalid");

            Factory = new WebApplicationFactory<Program>()
                .WithWebHostBuilder(b => b.UseEnvironment("Test"));
            Client = Factory.CreateClient();
        }

        public async Task DisposeAsync()
        {
            Client?.Dispose();
            if (Factory is not null)
            {
                await Factory.DisposeAsync();
            }
            await _postgres.DisposeAsync();
        }
    }
}
