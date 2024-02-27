package handlers_test

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/guidewire/fern-reporter/pkg/api/handlers"
	"github.com/guidewire/fern-reporter/pkg/models"
)

var (
	db     *sql.DB
	gormDb *gorm.DB
	mock   sqlmock.Sqlmock
)

var _ = BeforeEach(func() {
	db, mock, _ = sqlmock.New()

	dialector := postgres.New(postgres.Config{
		DSN:                  "sqlmock_db_0",
		DriverName:           "postgres",
		Conn:                 db,
		PreferSimpleProtocol: true,
	})
	gormDb, _ = gorm.Open(dialector, &gorm.Config{})

})

var _ = AfterEach(func() {
	db.Close()
})

// Define a custom type that implements the driver.Valuer and sql.Scanner interfaces
type myTime time.Time

// Implement the driver.Valuer interface
func (mt myTime) Value() (driver.Value, error) {
	return time.Time(mt), nil
}

// Implement the sql.Scanner interface
func (mt *myTime) Scan(value interface{}) error {
	*mt = myTime(value.(time.Time))
	return nil
}

var _ = Describe("Handlers", func() {
	Context("when GetTestRunAll handler is invoked", func() {
		It("should query db to fetch all records", func() {

			rows := sqlmock.NewRows([]string{"ID", "TestProjectName"}).
				AddRow(1, "project 1").
				AddRow(2, "project 2")

			mock.ExpectQuery("SELECT (.+) FROM \"test_runs\"").
				WithoutArgs().
				WillReturnRows(rows)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			handler := handlers.NewHandler(gormDb)

			handler.GetTestRunAll(c)

			Expect(w.Code).To(Equal(200))

			var testRuns []models.TestRun
			if err := json.NewDecoder(w.Body).Decode(&testRuns); err != nil {
				Fail(err.Error())
			}
			Expect(len(testRuns)).To(Equal(2))
			Expect(testRuns[0].TestProjectName).To(Equal("project 1"))
			Expect(testRuns[1].TestProjectName).To(Equal("project 2"))
		})
	})

	Context("When GetTestRunByID handler is invoked", func() {
		It("should query DB with where clause filtering by id", func() {

			rows := sqlmock.NewRows([]string{"ID", "TestProjectName"}).
				AddRow(123, "project 123")

			mock.ExpectQuery("SELECT (.+) FROM \"test_runs\" WHERE id = \\$1").
				WithArgs("123").
				WillReturnRows(rows)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Params = append(c.Params, gin.Param{Key: "id", Value: "123"})
			handler := handlers.NewHandler(gormDb)
			handler.GetTestRunByID(c)

			Expect(w.Code).To(Equal(200))

			var testRun models.TestRun

			if err := json.NewDecoder(w.Body).Decode(&testRun); err != nil {
				Fail(err.Error())
			}
			Expect(int(testRun.ID)).To(Equal(123))
			Expect(testRun.TestProjectName).To(Equal("project 123"))
		})
	})

	Context("when UpdateTestRun handler is invoked and test run does not exist", func() {
		It("should return 404", func() {
			rows := sqlmock.NewRows([]string{"ID", "TestProjectName"})
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM \"test_runs\" WHERE id = $1 ORDER BY \"test_runs\".\"id\" LIMIT 1")).
				WithArgs("123").
				WillReturnRows(rows)

			// Bind JSON data to gin Context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Params = append(c.Params, gin.Param{Key: "id", Value: "123"})
			handler := handlers.NewHandler(gormDb)
			handler.UpdateTestRun(c)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("when UpdateTestRun handler is invoked and test run exists", func() {
		It("should return 200 OK", func() {

			expectedTestRun := models.TestRun{
				ID:              1,
				TestProjectName: "TestProject",
				StartTime:       time.Now(),
				EndTime:         time.Now(),
				SuiteRuns: []models.SuiteRun{
					{
						ID:        1,
						TestRunID: 1,
						SuiteName: "TestSuite",
						StartTime: time.Now(),
						EndTime:   time.Now(),
						SpecRuns: []models.SpecRun{
							{
								ID:              1,
								SuiteID:         1,
								SpecDescription: "TestSpec",
								Status:          "Passed",
								Message:         "",
								StartTime:       time.Now(),
								EndTime:         time.Now(),
							},
						},
					},
				},
			}

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "test_runs" WHERE id = $1 ORDER BY "test_runs"."id" LIMIT 1`)).
				WithArgs("1").
				WillReturnRows(mock.NewRows([]string{"id", "test_project_name", "test_seed"}).
					AddRow(expectedTestRun.ID, expectedTestRun.TestProjectName, 1))

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Create a new request with JSON payload
			jsonStr := []byte(`{"id": 1, "test_project_name":"Updated Project"}`)
			req, err := http.NewRequest("POST", "/endpoint", bytes.NewBuffer(jsonStr))
			if err != nil {
				fmt.Printf("%v", err)
			}

			// Set the Content-Type header to application/json
			req.Header.Set("Content-Type", "application/json")

			c.Request = req
			c.Params = append(c.Params, gin.Param{Key: "id", Value: "1"})
			handler := handlers.NewHandler(gormDb)
			handler.UpdateTestRun(c)

			// Check the response status code
			Expect(w.Code).To(Equal(http.StatusOK))

		})
	})

	Context("when UpdateTestRun handler is invoked with bad POST payload", func() {
		It("should return error", func() {

			expectedTestRun := models.TestRun{
				ID:              1,
				TestProjectName: "TestProject",
				StartTime:       time.Now(),
				EndTime:         time.Now(),
			}

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "test_runs" WHERE id = $1 ORDER BY "test_runs"."id" LIMIT 1`)).
				WithArgs("1").
				WillReturnRows(mock.NewRows([]string{"id", "test_project_name", "test_seed", "start_time", "end_time"}).
					AddRow(expectedTestRun.ID, expectedTestRun.TestProjectName, expectedTestRun.TestSeed, expectedTestRun.StartTime, expectedTestRun.EndTime))

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Create a new request with JSON payload
			jsonStr := []byte(`{"BAD_PAYLOAD_KEY": "BAD_VALUE"}`)

			req, err := http.NewRequest("POST", "/endpoint", bytes.NewBuffer(jsonStr))
			if err != nil {
				fmt.Printf("%v", err)
			}

			// Set the Content-Type header to application/json
			req.Header.Set("Content-Type", "application/json")

			c.Request = req
			c.Params = append(c.Params, gin.Param{Key: "id", Value: "1"})
			handler := handlers.NewHandler(gormDb)
			handler.UpdateTestRun(c)

			// Create a map to represent the response
			var responseBody models.TestRun
			err = json.Unmarshal(w.Body.Bytes(), &responseBody)

			Expect(err).ToNot(HaveOccurred())
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(expectedTestRun.ID).To(Equal(responseBody.ID))
			Expect(expectedTestRun.SuiteRuns).To(Equal(responseBody.SuiteRuns))
			Expect(expectedTestRun.TestProjectName).To(Equal(responseBody.TestProjectName))
			Expect(expectedTestRun.StartTime).To(BeTemporally("==", responseBody.StartTime))
			Expect(expectedTestRun.EndTime).To(BeTemporally("==", responseBody.EndTime))
			Expect(expectedTestRun.TestSeed).To(Equal(responseBody.TestSeed))

		})
	})

	Context("When DeleteTestRun handler is invoked", func() {
		It("should delete record from DB by id", func() {

			testRunRow := sqlmock.NewResult(1, 1)

			mock.ExpectBegin()
			mock.ExpectExec("DELETE FROM \"test_runs\" WHERE \"test_runs\".\"id\" = \\$1").
				WithArgs(123).
				WillReturnResult(testRunRow)
			mock.ExpectCommit()
			mock.ExpectClose()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Params = append(c.Params, gin.Param{Key: "id", Value: "123"})
			handler := handlers.NewHandler(gormDb)
			handler.DeleteTestRun(c)

			Expect(w.Code).To(Equal(200))

			var testRun models.TestRun

			if err := json.NewDecoder(w.Body).Decode(&testRun); err != nil {
				Fail(err.Error())
			}
			Expect(int(testRun.ID)).To(Equal(123))
		})

		It("should handle error", func() {

			mock.ExpectBegin()
			mock.ExpectExec("DELETE FROM \"test_runs\" WHERE \"test_runs\".\"id\" = \\$1").
				WithArgs(123).
				WillReturnError(sql.ErrConnDone)
			mock.ExpectRollback()
			mock.ExpectClose()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Params = append(c.Params, gin.Param{Key: "id", Value: "123"})
			handler := handlers.NewHandler(gormDb)
			handler.DeleteTestRun(c)

			Expect(w.Code).To(Not(Equal(200)))
			Expect(w.Code).To((Equal(http.StatusInternalServerError)))

			var testRun models.TestRun

			if err := json.NewDecoder(w.Body).Decode(&testRun); err != nil {
				Fail(err.Error())
			}
		})

		It("should handle scenario of no rows affected", func() {

			mock.ExpectBegin()
			mock.ExpectExec("DELETE FROM \"test_runs\" WHERE \"test_runs\".\"id\" = \\$1").
				WithArgs(123).
				WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectCommit()
			mock.ExpectClose()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Params = append(c.Params, gin.Param{Key: "id", Value: "123"})
			handler := handlers.NewHandler(gormDb)
			handler.DeleteTestRun(c)

			Expect(w.Code).To((Equal(http.StatusNotFound)))

			// Ensure you call Result before reading the body
			result := w.Result()

			// Extract the response body as a string
			body, err := io.ReadAll(result.Body)
			if err != nil {
				// Handle the error
				fmt.Printf("Error reading response body: %v", err)
				return
			}

			// Parse the JSON response
			var response map[string]interface{}
			if err := json.Unmarshal(body, &response); err != nil {
				// Handle the error
				fmt.Printf("Error parsing JSON response: %v", err)
				return
			}

			// Extract the error message
			errorMessage, _ := response["error"].(string)
			Expect(errorMessage).To((Equal("test run not found")))

		})

		It("should handle invalid id format", func() {
			mock.ExpectBegin()
			mock.ExpectExec("DELETE FROM \"test_runs\" WHERE \"test_runs\".\"id\" = \\$1").
				WithArgs(123).
				WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectCommit()
			mock.ExpectClose()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Params = append(c.Params, gin.Param{Key: "id", Value: "invalidID"})
			handler := handlers.NewHandler(gormDb)
			handler.DeleteTestRun(c)

			Expect(w.Code).To((Equal(http.StatusNotFound)))

		})

	})

	//Context("When ReportTestRunById handler is invoked", func() {
	//	It("should query DB with where clause filtering by id", func() {
	//
	//		expectedTestRun := models.TestRun{
	//			ID:              123,
	//			TestProjectName: "TestProject",
	//			StartTime:       time.Now(),
	//			EndTime:         time.Now(),
	//			SuiteRuns: []models.SuiteRun{
	//				{
	//					ID:        1,
	//					TestRunID: 123,
	//					SuiteName: "TestSuite",
	//					StartTime: time.Now(),
	//					EndTime:   time.Now(),
	//					SpecRuns: []models.SpecRun{
	//						{
	//							ID:              1,
	//							SuiteID:         1,
	//							SpecDescription: "TestSpec",
	//							Status:          "Passed",
	//							Message:         "",
	//							StartTime:       time.Now(),
	//							EndTime:         time.Now(),
	//						},
	//					},
	//				},
	//			},
	//		}
	//
	//		// Mock database query
	//		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "test_runs" WHERE id = $1 ORDER BY "test_runs"."id" LIMIT 1`)).
	//			WithArgs("123").
	//			WillReturnRows(mock.NewRows([]string{"id", "test_project_name", "start_time", "end_time"}).
	//				AddRow(expectedTestRun.ID, expectedTestRun.TestProjectName, expectedTestRun.StartTime, expectedTestRun.EndTime))
	//
	//		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."test_run_id" = $1`)).
	//			WithArgs(123).
	//			WillReturnRows(mock.NewRows([]string{"id", "test_run_id", "suite_name", "start_time", "end_time"}).
	//				AddRow(expectedTestRun.SuiteRuns[0].ID, expectedTestRun.ID, expectedTestRun.SuiteRuns[0].SuiteName, expectedTestRun.SuiteRuns[0].StartTime, expectedTestRun.SuiteRuns[0].EndTime))
	//
	//		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE "spec_runs"."suite_id" = $1`)).
	//			WithArgs(1).
	//			WillReturnRows(mock.NewRows([]string{"id", "suite_id", "spec_description", "status", "start_time", "end_time"}).
	//				AddRow(expectedTestRun.SuiteRuns[0].SpecRuns[0].ID, expectedTestRun.SuiteRuns[0].ID, expectedTestRun.SuiteRuns[0].SpecRuns[0].SpecDescription, expectedTestRun.SuiteRuns[0].SpecRuns[0].Status, expectedTestRun.SuiteRuns[0].SpecRuns[0].StartTime, expectedTestRun.SuiteRuns[0].SpecRuns[0].EndTime))
	//
	//		// Call the method
	//		w := httptest.NewRecorder()
	//		c, _ := gin.CreateTestContext(w)
	//		//gin.ena
	//		//
	//		c.Params = append(c.Params, gin.Param{Key: "id", Value: "123"})
	//		handler := handlers.NewHandler(gormDb)
	//
	//		handler.ReportTestRunById(c)
	//
	//		// Verify the response
	//		//require.Equal(t, http.StatusOK, c.Writer.Status())
	//
	//		// Verify the HTML output if needed
	//		// htmlOutput := c.Writer.Body.String()
	//		// require.Contains(t, htmlOutput, "Expected HTML output")
	//
	//		// Check for any remaining expectations
	//		//require.NoError(t, mock.ExpectationsWereMet())
	//
	//		//specRunRow := sqlmock.NewRows([]string{"id", "suite_id", "spec_description", "status", "message", "start_time", "end_time"}).
	//		//	AddRow(1, 11, "desc1", "status", "message", myTime(time.Now()), myTime(time.Now()))
	//		//mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE "spec_runs"."suite_id" IN ($1)`)).WithArgs(1).WillReturnRows(specRunRow)
	//		//
	//		//suiteRunRow := sqlmock.NewRows([]string{"id", "test_run_id", "suite_name", "start_time", "end_time"}).
	//		//	AddRow(1, 123, "Sample Spec 1", myTime(time.Now()), myTime(time.Now())).
	//		//	AddRow(2, 123, "Sample Spec 2", myTime(time.Now()), myTime(time.Now()))
	//		//
	//		//mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."test_run_id" = $1`)).WithArgs(123).WillReturnRows(suiteRunRow)
	//		//
	//		//rows := sqlmock.NewRows([]string{"ID", "TestProjectName"}).
	//		//	AddRow(123, "project 1")
	//		//
	//		//mock.ExpectQuery(regexp.QuoteMeta(`SELECT \* FROM "spec_runs" WHERE "spec_runs"\."suite_id" IN \(\$1\)`)).
	//		//	WithArgs("123").
	//		//	WillReturnRows(rows)
	//		//
	//		//w := httptest.NewRecorder()
	//		//c, _ := gin.CreateTestContext(w)
	//		//
	//		//c.Params = append(c.Params, gin.Param{Key: "id", Value: "123"})
	//		//handler := handlers.NewHandler(gormDb)
	//		//handler.ReportTestRunById(c)
	//		//
	//		//Expect(w.Code).To(Equal(200))
	//		//
	//		//var testRun models.TestRun
	//		//
	//		//if err := json.NewDecoder(w.Body).Decode(&testRun); err != nil {
	//		//	Fail(err.Error())
	//		//}
	//		//Expect(int(testRun.ID)).To(Equal(123))
	//		//Expect(testRun.TestProjectName).To(Equal("project 123"))
	//	})
	//})
})
