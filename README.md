# AWS Lambda Handler

Provides a wrapper method (with logging and XRay tracing) to start a lambda function.

## Usage:

```go
import "github.com/ockendenjo/handler"

func main() {
    handler.BuildAndStart(func(awsConfig aws.Config) handler.Handler[events.APIGatewayProxyRequest, events.APIGatewayProxyResponse] {
        //Set up any AWS SDK clients here (using awsConfig)
        
        return func(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
            //This function is invoked for every lambda invocation
            response := events.APIGatewayProxyResponse{
                StatusCode: 200,
            }
            return response, nil
        }
    })
}

```
