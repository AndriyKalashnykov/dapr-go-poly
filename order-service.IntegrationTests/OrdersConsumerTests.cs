using System.Text.Json;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging.Abstractions;
using RabbitMQ.Client;
using Testcontainers.PostgreSql;
using Testcontainers.RabbitMq;
using TUnit.Assertions;
using TUnit.Assertions.Extensions;
using TUnit.Core;
using TUnit.Core.Interfaces;

namespace order_service.IntegrationTests;

/// <summary>
/// Integration test for OrdersConsumer: publish to the real RabbitMQ "orders"
/// queue and assert the consumer persists the Order to Postgres.
/// </summary>
[Category("Integration")]
public sealed class OrdersConsumerTests : IAsyncInitializer, IAsyncDisposable
{
    private readonly PostgreSqlContainer _postgres = new PostgreSqlBuilder("postgres:18-alpine")
        .WithDatabase("order")
        .WithUsername("postgres")
        .WithPassword("test")
        .Build();

    private readonly RabbitMqContainer _rabbit = new RabbitMqBuilder("rabbitmq:4-management-alpine")
        .WithUsername("guest")
        .WithPassword("guest")
        .Build();

    public async Task InitializeAsync()
    {
        await Task.WhenAll(_postgres.StartAsync(), _rabbit.StartAsync());
    }

    public async ValueTask DisposeAsync()
    {
        await Task.WhenAll(_postgres.DisposeAsync().AsTask(), _rabbit.DisposeAsync().AsTask());
    }

    [Test]
    public async Task ReceivedOrder_IsPersistedToDatabase()
    {
        Environment.SetEnvironmentVariable("RABBITMQ_HOST", _rabbit.Hostname);
        Environment.SetEnvironmentVariable("RABBITMQ_PORT", _rabbit.GetMappedPublicPort(5672).ToString());
        Environment.SetEnvironmentVariable("RABBITMQ_USERNAME", "guest");
        Environment.SetEnvironmentVariable("RABBITMQ_PASSWORD", "guest");

        var connString = $"Host={_postgres.Hostname};Port={_postgres.GetMappedPublicPort(5432)};" +
                         "Database=order;Username=postgres;Password=test";

        var services = new ServiceCollection();
        services.AddLogging();
        services.AddDbContext<OrderContext>(o => o.UseNpgsql(connString));
        using var provider = services.BuildServiceProvider();

        using (var scope = provider.CreateScope())
        {
            var db = scope.ServiceProvider.GetRequiredService<OrderContext>();
            await db.Database.EnsureCreatedAsync();
        }

        var consumer = new OrdersConsumer(NullLogger<OrdersConsumer>.Instance, provider);
        var cts = new CancellationTokenSource();
        var task = consumer.StartAsync(cts.Token);

        await Task.Delay(TimeSpan.FromSeconds(2));

        var order = new Order { Id = Guid.NewGuid(), ProductId = Guid.NewGuid(), Quantity = 3u, Price = 12.50m };
        var body = JsonSerializer.SerializeToUtf8Bytes(order);
        PublishToOrdersQueue(body);

        var deadline = DateTime.UtcNow.AddSeconds(20);
        Order? stored = null;
        while (DateTime.UtcNow < deadline)
        {
            using var scope = provider.CreateScope();
            var db = scope.ServiceProvider.GetRequiredService<OrderContext>();
            stored = await db.Orders.FindAsync(order.Id);
            if (stored is not null) break;
            await Task.Delay(500);
        }

        await cts.CancelAsync();
        try { await task; } catch (OperationCanceledException) { }

        await Assert.That(stored).IsNotNull();
        await Assert.That(stored!.ProductId).IsEqualTo(order.ProductId);
        await Assert.That(stored.Quantity).IsEqualTo(3u);
        await Assert.That(stored.Price).IsEqualTo(12.50m);
    }

    private void PublishToOrdersQueue(byte[] body)
    {
        var factory = new ConnectionFactory
        {
            HostName = _rabbit.Hostname,
            Port = _rabbit.GetMappedPublicPort(5672),
            UserName = "guest",
            Password = "guest",
        };

        using var connection = factory.CreateConnectionAsync().GetAwaiter().GetResult();
        using var channel = connection.CreateChannelAsync().GetAwaiter().GetResult();

        channel.QueueDeclareAsync(queue: "orders", durable: true, exclusive: false, autoDelete: false, arguments: null)
            .GetAwaiter().GetResult();

        channel.BasicPublishAsync(exchange: "", routingKey: "orders", body: body).GetAwaiter().GetResult();
    }
}
