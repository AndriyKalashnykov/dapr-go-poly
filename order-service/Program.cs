using Microsoft.EntityFrameworkCore;

var builder = WebApplication.CreateBuilder(args);

var dbHost = Environment.GetEnvironmentVariable("POSTGRES_HOST");
var dbPort = Environment.GetEnvironmentVariable("POSTGRES_PORT");
var dbName = Environment.GetEnvironmentVariable("POSTGRES_DATABASE");
var dbUsername = Environment.GetEnvironmentVariable("POSTGRES_USERNAME");
var dbPassword = Environment.GetEnvironmentVariable("POSTGRES_PASSWORD");

string connectionString = $"Host={dbHost};Port={dbPort};Database={dbName};Username={dbUsername};Password={dbPassword}";

builder.Services.AddDbContext<OrderContext>(options => options.UseNpgsql(connectionString));

builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen();

builder.Services.AddHttpClient<IProductsClient, ProductsClient>(client =>
{
    client.BaseAddress = new Uri(Environment.GetEnvironmentVariable("PRODUCT_SERVICE_BASE_URL") ?? "");
});

// OrdersConsumer is a BackgroundService that consumes from RabbitMQ "orders"
// queue and persists Orders to Postgres. It declares the queue on startup,
// so must be registered as a HostedService for the async pipeline to run.
builder.Services.AddHostedService<OrdersConsumer>();

var app = builder.Build();

app.MapSwagger();
app.UseSwaggerUI();

using (var scope = app.Services.CreateScope())
{
    var context = scope.ServiceProvider.GetRequiredService<OrderContext>();
    await context.Database.EnsureCreatedAsync();
}

app.UseStatusCodePages(async statusCodeContext
    => await Results.Problem(statusCode: statusCodeContext.HttpContext.Response.StatusCode)
                 .ExecuteAsync(statusCodeContext.HttpContext));

app.MapGet("/api/orders", (OrderContext db) => db.Orders.ToListAsync());

app.Run();

public partial class Program { }
