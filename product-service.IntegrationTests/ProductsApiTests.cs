using System.Net;
using System.Net.Http.Json;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.Hosting;
using Testcontainers.PostgreSql;
using Xunit;

namespace product_service.IntegrationTests;

[Trait("Category", "Integration")]
public sealed class ProductsApiTests : IAsyncLifetime, IClassFixture<ProductsApiTests.Fixture>
{
    private readonly Fixture _fixture;

    public ProductsApiTests(Fixture fixture) => _fixture = fixture;

    public Task InitializeAsync() => Task.CompletedTask;
    public Task DisposeAsync() => Task.CompletedTask;

    [Fact]
    public async Task Post_CreatesProduct_AndRoundTripsViaGet()
    {
        var client = _fixture.Client;
        var payload = new { Name = "Widget", Description = "A widget for testing", Price = 9.99m };

        var createResp = await client.PostAsJsonAsync("/api/products", payload);
        Assert.Equal(HttpStatusCode.Created, createResp.StatusCode);
        var created = await createResp.Content.ReadFromJsonAsync<ProductDto>();
        Assert.NotNull(created);
        Assert.NotEqual(Guid.Empty, created!.Id);
        Assert.Equal("Widget", created.Name);
        Assert.Equal(9.99m, created.Price);

        var getResp = await client.GetAsync($"/api/products/{created.Id}");
        Assert.Equal(HttpStatusCode.OK, getResp.StatusCode);
        var fetched = await getResp.Content.ReadFromJsonAsync<ProductDto>();
        Assert.Equal(created.Id, fetched!.Id);
        Assert.Equal("A widget for testing", fetched.Description);
    }

    [Fact]
    public async Task Put_UpdatesExistingProduct()
    {
        var client = _fixture.Client;
        var created = await CreateProductAsync(client, "Gadget", "Initial", 5.00m);

        var updateResp = await client.PutAsJsonAsync(
            $"/api/products/{created.Id}",
            new { Name = "Gadget Pro", Description = "Updated", Price = 12.50m });
        Assert.Equal(HttpStatusCode.NoContent, updateResp.StatusCode);

        var fetched = await client.GetFromJsonAsync<ProductDto>($"/api/products/{created.Id}");
        Assert.Equal("Gadget Pro", fetched!.Name);
        Assert.Equal(12.50m, fetched.Price);
    }

    [Fact]
    public async Task Delete_RemovesProduct_AndSubsequentGetReturns404()
    {
        var client = _fixture.Client;
        var created = await CreateProductAsync(client, "Disposable", "Temp", 1.00m);

        var delResp = await client.DeleteAsync($"/api/products/{created.Id}");
        Assert.Equal(HttpStatusCode.NoContent, delResp.StatusCode);

        var getResp = await client.GetAsync($"/api/products/{created.Id}");
        Assert.Equal(HttpStatusCode.NotFound, getResp.StatusCode);
    }

    [Fact]
    public async Task Get_ReturnsNotFound_ForUnknownId()
    {
        var client = _fixture.Client;
        var resp = await client.GetAsync($"/api/products/{Guid.NewGuid()}");
        Assert.Equal(HttpStatusCode.NotFound, resp.StatusCode);
    }

    [Fact]
    public async Task Get_ListEndpoint_ReturnsAllSeededProducts()
    {
        var client = _fixture.Client;

        var a = await CreateProductAsync(client, "List-A-" + Guid.NewGuid(), "First seed", 1.00m);
        var b = await CreateProductAsync(client, "List-B-" + Guid.NewGuid(), "Second seed", 2.00m);

        var listResp = await client.GetAsync("/api/products");
        Assert.Equal(HttpStatusCode.OK, listResp.StatusCode);
        var all = await listResp.Content.ReadFromJsonAsync<List<ProductDto>>();
        Assert.NotNull(all);
        Assert.Contains(all!, p => p.Id == a.Id);
        Assert.Contains(all!, p => p.Id == b.Id);
    }

    [Theory]
    [InlineData("", "ok description", 9.99, "Name")]
    [InlineData("ok name", "", 9.99, "Description")]
    [InlineData("ok name", "ok description", 0, "Price")]
    [InlineData("ok name", "ok description", -1, "Price")]
    public async Task Post_RejectsInvalidInput(string name, string description, decimal price, string expectedField)
    {
        var client = _fixture.Client;
        var resp = await client.PostAsJsonAsync(
            "/api/products",
            new { Name = name, Description = description, Price = price });
        Assert.Equal(HttpStatusCode.BadRequest, resp.StatusCode);
        var body = await resp.Content.ReadAsStringAsync();
        Assert.Contains(expectedField, body, StringComparison.OrdinalIgnoreCase);
    }

    private static async Task<ProductDto> CreateProductAsync(HttpClient client, string name, string description, decimal price)
    {
        var resp = await client.PostAsJsonAsync("/api/products", new { Name = name, Description = description, Price = price });
        resp.EnsureSuccessStatusCode();
        return (await resp.Content.ReadFromJsonAsync<ProductDto>())!;
    }

    public sealed record ProductDto(Guid Id, string Name, string Description, decimal Price);

    public sealed class Fixture : IAsyncLifetime
    {
        private readonly PostgreSqlContainer _postgres = new PostgreSqlBuilder()
            .WithImage("postgres:17-alpine")
            .WithDatabase("product")
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
            Environment.SetEnvironmentVariable("POSTGRES_DATABASE", "product");
            Environment.SetEnvironmentVariable("POSTGRES_USERNAME", "postgres");
            Environment.SetEnvironmentVariable("POSTGRES_PASSWORD", "test");

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
