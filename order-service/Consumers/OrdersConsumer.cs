using System.Text.Json;
using RabbitMQ.Client;
using RabbitMQ.Client.Events;

// https://www.rabbitmq.com/tutorials/tutorial-one-dotnet

public class OrdersConsumer : BackgroundService
{
    private readonly ILogger<OrdersConsumer> _logger;
    private readonly IServiceProvider _serviceProvider;
    private string? rabbitMQServer = Environment.GetEnvironmentVariable("RABBITMQ_HOST");
    private string? rabbitMQPort = Environment.GetEnvironmentVariable("RABBITMQ_PORT");
    private string? rabbitMQUser = Environment.GetEnvironmentVariable("RABBITMQ_USERNAME");
    private string? rabbitMQPass = Environment.GetEnvironmentVariable("RABBITMQ_PASSWORD");
    private readonly string queueName = "orders";

    public OrdersConsumer(ILogger<OrdersConsumer> logger, IServiceProvider serviceProvider)
    {
        _logger = logger;
        _serviceProvider = serviceProvider;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        stoppingToken.ThrowIfCancellationRequested();

        var db = _serviceProvider.CreateScope().ServiceProvider.GetRequiredService<OrderContext>();

        int port;
        int.TryParse(rabbitMQPort, out port);

        var factory = new ConnectionFactory()
        {
            HostName = rabbitMQServer ?? "localhost",
            Port = port,
            UserName = rabbitMQUser ?? "guest",
            Password = rabbitMQPass ?? "guest"
        };

        // Reconnect loop with exponential backoff. RabbitMQ's startup races
        // with our Docker Compose healthcheck (`rabbitmq-diagnostics ping`
        // returns OK before the routing step finishes); a single connect
        // attempt on cold-boot often hits "None of the specified endpoints
        // were reachable" and the consumer silently gives up. The compose
        // healthcheck was tightened (check_running) but we keep a retry here
        // for resilience against any other transient brokerwide issue.
        IConnection? connection = null;
        var connectDelay = TimeSpan.FromSeconds(1);
        for (var attempt = 1; attempt <= 10; attempt++)
        {
            try
            {
                connection = await factory.CreateConnectionAsync(stoppingToken);
                break;
            }
            catch (OperationCanceledException)
            {
                return;
            }
            catch (Exception ex)
            {
                _logger.LogWarning("RabbitMQ connect attempt {Attempt} failed: {Message}; retrying in {Delay}",
                    attempt, ex.Message, connectDelay);
                try { await Task.Delay(connectDelay, stoppingToken); }
                catch (OperationCanceledException) { return; }
                connectDelay = TimeSpan.FromMilliseconds(Math.Min(connectDelay.TotalMilliseconds * 2, 15_000));
            }
        }
        if (connection is null)
        {
            _logger.LogError("Giving up connecting to RabbitMQ after 10 attempts");
            return;
        }

        try
        {
            using var _conn = connection;
            using var channel = await connection.CreateChannelAsync();

            var c = await channel.QueueDeclareAsync(queue: queueName,
                                    durable: true,
                                    exclusive: false,
                                    autoDelete: false,
                                    arguments: null);

            var consumer = new AsyncEventingBasicConsumer(channel);

            _logger.LogInformation("READY TO RECEIVE");

            consumer.ReceivedAsync += async (model, ea) =>
            {
                _logger.LogInformation("RECEIVED");

                var body = ea.Body.ToArray();
                try
                {
                    var request = JsonSerializer.Deserialize<Order>(body);
                    if (request is null)
                    {
                        return;
                    }

                    await db.Orders.AddAsync(request);
                    await db.SaveChangesAsync();
                }
                catch (Exception ex)
                {
                    _logger.LogError($"Error while processing order: {ex.Message} {ex.StackTrace}");
                }
            };

            await channel.BasicConsumeAsync(queue: c.QueueName,
                autoAck: true,
                consumer: consumer);

            // Keep connection/channel alive until shutdown; without this the
            // `using` scopes dispose them immediately and no messages flow.
            await Task.Delay(Timeout.Infinite, stoppingToken);
        }
        catch (OperationCanceledException)
        {
            // Expected on graceful shutdown.
        }
        catch (Exception ex)
        {
            _logger.LogError($"Error while consuming from RabbitMQ: '{ex.Message}' {ex.StackTrace}");
        }
    }
}