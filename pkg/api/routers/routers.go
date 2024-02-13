package routers

import (
	"gin_graphql/graph"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/guidewire/fern-reporter/pkg/api/handlers"
	"github.com/guidewire/fern-reporter/pkg/graph/generated"

	"github.com/gin-gonic/gin"
)

func RegisterRouters(router *gin.Engine) {
	// router.GET("/", handlers.Home)
	api := router.Group("/api")
	{
		testRun := api.Group("/testrun")
		testRun.GET("/", handlers.GetTestRunAll)
		testRun.GET("/:id", handlers.GetTestRunByID)
		testRun.POST("/", handlers.CreateTestRun)
		testRun.PUT("/:id", handlers.UpdateTestRun)
		testRun.DELETE("/:id", handlers.DeleteTestRun)
		testRun.GET("/_graphql", PlaygroundHandler("/query"))
	}
	reports := router.Group("/reports/testruns")
	{
		testRunReport := reports.GET("/", handlers.ReportTestRunAll)
		testRunReport.GET("/:id", handlers.ReportTestRunById)
	}
}

func PlaygroundHandler(path string) gin.HandlerFunc {
	h := playground.Handler("GraphQL playground", path)
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func GraphqlHandler() gin.HandlerFunc {
	c := generated.Config{Resolvers: &graph.Resolver{}}
	// Schema Directive

	// srv := glmiddleware.AuthMiddleware(handler.NewDefaultServer(generated.NewExecutableSchema(c)))
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(c))

	return func(c *gin.Context) {
		srv.ServeHTTP(c.Writer, c.Request)
	}
}
