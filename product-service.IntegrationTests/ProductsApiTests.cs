using System.Net;
using System.Net.Http.Json;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Testcontainers.PostgreSql;
using TUnit.Assertions;
using TUnit.Assertions.Extensions;
using TUnit.Core;
using TUnit.Core.Interfaces;

namespace product_service.IntegrationTests;

/// <summary>
/// Shared fixture: one PostgreSQL container + WebApplicationFactory per test
/// class. TUnit's [ClassDataSource&lt;T&gt;(Shared = SharedType.PerClass)]
/// replaces xUnit's IClassFixture&lt;T&gt;.
/// </summary>
public sealed class ProductsApiFixture : IAsyncInitializer, IAsyncDisposable
{
    public PostgreSqlContainer Postgres { get; } = new PostgreSqlBuilder()
        .WithImage("postgres:17-alpine")
        .WithDatabase("product")
        .WithUsername("postgres")
        .WithPassword("test")
        .Build();

    public WebApplicationFactory<Program> Factory { get; private set; } = null!;

    public async Task InitializeAsync()
    {
        await Postgres.StartAsync();

        Environment.SetEnvironmentVariable("POSTGRES_HOST", Postgres.Hostname);
        Environment.SetEnvironmentVariable("POSTGRES_PORT", Postgres.GetMappedPublicPort(5432).ToString());
        Environment.SetEnvironmentVariable("POSTGRES_DATABASE", "product");
        Environment.SetEnvironmentVariable("POSTGRES_USERNAME", "postgres");
        Environment.SetEnvironmentVariable("POSTGRES_PASSWORD", "test");

        Factory = new WebApplicationFactory<Program>()
            .WithWebHostBuilder(b => b.UseEnvironment("Test"));
    }

    public async ValueTask DisposeAsync()
    {
        await Factory.DisposeAsync();
        await Postgres.DisposeAsync();
    }
}

[ClassDataSource<ProductsApiFixture>(Shared = SharedType.PerClass)]
[Category("Integration")]
public sealed class ProductsApiTests(ProductsApiFixture fixture)
{
    [Test]
    public async Task Post_CreatesProduct_AndRoundTripsViaGet()
    {
        using var client = fixture.Factory.CreateClient();
        var payload = new { Name = "Widget", Description = "A widget for testing", Price = 9.99m };

        var createResp = await client.PostAsJsonAsync("/api/products", payload);
        await Assert.That(createResp.StatusCode).IsEqualTo(HttpStatusCode.Created);

        var created = await createResp.Content.ReadFromJsonAsync<ProductDto>();
        await Assert.That(created).IsNotNull();
        await Assert.That(created!.Id).IsNotEqualTo(Guid.Empty);
        await Assert.That(created.Name).IsEqualTo("Widget");
        await Assert.That(created.Price).IsEqualTo(9.99m);

        var getResp = await client.GetAsync($"/api/products/{created.Id}");
        await Assert.That(getResp.StatusCode).IsEqualTo(HttpStatusCode.OK);
        var fetched = await getResp.Content.ReadFromJsonAsync<ProductDto>();
        await Assert.That(fetched!.Id).IsEqualTo(created.Id);
        await Assert.That(fetched.Description).IsEqualTo("A widget for testing");
    }

    [Test]
    public async Task Put_UpdatesExistingProduct()
    {
        using var client = fixture.Factory.CreateClient();
        var created = await CreateProductAsync(client, "Gadget", "Initial", 5.00m);

        var updateResp = await client.PutAsJsonAsync(
            $"/api/products/{created.Id}",
            new { Name = "Gadget Pro", Description = "Updated", Price = 12.50m });
        await Assert.That(updateResp.StatusCode).IsEqualTo(HttpStatusCode.NoContent);

        var fetched = await client.GetFromJsonAsync<ProductDto>($"/api/products/{created.Id}");
        await Assert.That(fetched!.Name).IsEqualTo("Gadget Pro");
        await Assert.That(fetched.Price).IsEqualTo(12.50m);
    }

    [Test]
    public async Task Delete_RemovesProduct_AndSubsequentGetReturns404()
    {
        using var client = fixture.Factory.CreateClient();
        var created = await CreateProductAsync(client, "Disposable", "Temp", 1.00m);

        var delResp = await client.DeleteAsync($"/api/products/{created.Id}");
        await Assert.That(delResp.StatusCode).IsEqualTo(HttpStatusCode.NoContent);

        var getResp = await client.GetAsync($"/api/products/{created.Id}");
        await Assert.That(getResp.StatusCode).IsEqualTo(HttpStatusCode.NotFound);
    }

    [Test]
    public async Task Get_ReturnsNotFound_ForUnknownId()
    {
        using var client = fixture.Factory.CreateClient();
        var resp = await client.GetAsync($"/api/products/{Guid.NewGuid()}");
        await Assert.That(resp.StatusCode).IsEqualTo(HttpStatusCode.NotFound);
    }

    [Test]
    public async Task Get_ListEndpoint_ReturnsAllSeededProducts()
    {
        using var client = fixture.Factory.CreateClient();

        var a = await CreateProductAsync(client, "List-A-" + Guid.NewGuid(), "First seed", 1.00m);
        var b = await CreateProductAsync(client, "List-B-" + Guid.NewGuid(), "Second seed", 2.00m);

        var listResp = await client.GetAsync("/api/products");
        await Assert.That(listResp.StatusCode).IsEqualTo(HttpStatusCode.OK);
        var all = await listResp.Content.ReadFromJsonAsync<List<ProductDto>>();
        await Assert.That(all).IsNotNull();
        await Assert.That(all!.Any(p => p.Id == a.Id)).IsTrue();
        await Assert.That(all.Any(p => p.Id == b.Id)).IsTrue();
    }

    [Test]
    [Arguments("", "ok description", 9.99, "Name")]
    [Arguments("ok name", "", 9.99, "Description")]
    [Arguments("ok name", "ok description", 0, "Price")]
    [Arguments("ok name", "ok description", -1, "Price")]
    public async Task Post_RejectsInvalidInput(string name, string description, double price, string expectedField)
    {
        using var client = fixture.Factory.CreateClient();
        var resp = await client.PostAsJsonAsync(
            "/api/products",
            new { Name = name, Description = description, Price = (decimal)price });
        await Assert.That(resp.StatusCode).IsEqualTo(HttpStatusCode.BadRequest);
        var body = await resp.Content.ReadAsStringAsync();
        await Assert.That(body.Contains(expectedField, StringComparison.OrdinalIgnoreCase)).IsTrue();
    }

    private static async Task<ProductDto> CreateProductAsync(HttpClient client, string name, string description, decimal price)
    {
        var resp = await client.PostAsJsonAsync("/api/products", new { Name = name, Description = description, Price = price });
        resp.EnsureSuccessStatusCode();
        return (await resp.Content.ReadFromJsonAsync<ProductDto>())!;
    }

    public sealed record ProductDto(Guid Id, string Name, string Description, decimal Price);
}
