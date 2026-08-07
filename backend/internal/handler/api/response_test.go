package api

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

func acquireCtx(app *fiber.App) fiber.Ctx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/")
	return app.AcquireCtx(ctx)
}

func TestErrorJSON(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := ErrorJSON(c, 400, "bad request", CodeBadRequest)
	if err != nil {
		t.Fatalf("ErrorJSON failed: %v", err)
	}
	if c.Response().StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response().StatusCode())
	}
}

func TestBadRequest(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := BadRequest(c, "invalid input")
	if err != nil {
		t.Fatalf("BadRequest failed: %v", err)
	}
	if c.Response().StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response().StatusCode())
	}
}

func TestUnauthorized(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := Unauthorized(c, "missing auth")
	if err != nil {
		t.Fatalf("Unauthorized failed: %v", err)
	}
	if c.Response().StatusCode() != 401 {
		t.Errorf("expected 401, got %d", c.Response().StatusCode())
	}
}

func TestForbidden(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := Forbidden(c, "no access")
	if err != nil {
		t.Fatalf("Forbidden failed: %v", err)
	}
	if c.Response().StatusCode() != 403 {
		t.Errorf("expected 403, got %d", c.Response().StatusCode())
	}
}

func TestNotFound(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := NotFound(c, "not found")
	if err != nil {
		t.Fatalf("NotFound failed: %v", err)
	}
	if c.Response().StatusCode() != 404 {
		t.Errorf("expected 404, got %d", c.Response().StatusCode())
	}
}

func TestConflict(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := Conflict(c, "already exists")
	if err != nil {
		t.Fatalf("Conflict failed: %v", err)
	}
	if c.Response().StatusCode() != 409 {
		t.Errorf("expected 409, got %d", c.Response().StatusCode())
	}
}

func TestInternalError(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := InternalError(c, "server error")
	if err != nil {
		t.Fatalf("InternalError failed: %v", err)
	}
	if c.Response().StatusCode() != 500 {
		t.Errorf("expected 500, got %d", c.Response().StatusCode())
	}
}

func TestSuccessOK(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := SuccessOK(c)
	if err != nil {
		t.Fatalf("SuccessOK failed: %v", err)
	}
	if c.Response().StatusCode() != 200 {
		t.Errorf("expected 200, got %d", c.Response().StatusCode())
	}
}

func TestInvalidBody(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := InvalidBody(c)
	if err != nil {
		t.Fatalf("InvalidBody failed: %v", err)
	}
	if c.Response().StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response().StatusCode())
	}
}

func TestMissingField(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := MissingField(c, "email")
	if err != nil {
		t.Fatalf("MissingField failed: %v", err)
	}
	if c.Response().StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response().StatusCode())
	}
}

func TestSuccessData(t *testing.T) {
	app := fiber.New()
	c := acquireCtx(app)
	defer app.ReleaseCtx(c)

	err := SuccessData(c, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("SuccessData failed: %v", err)
	}
	if c.Response().StatusCode() != 200 {
		t.Errorf("expected 200, got %d", c.Response().StatusCode())
	}
}

func TestErrorStruct(t *testing.T) {
	e := Error{Error: "test", Code: "TEST_ERROR", Details: "details"}
	if e.Error != "test" {
		t.Errorf("expected 'test', got '%s'", e.Error)
	}
	if e.Code != "TEST_ERROR" {
		t.Errorf("expected 'TEST_ERROR', got '%s'", e.Code)
	}
}

func TestSuccessStruct(t *testing.T) {
	s := Success{Status: "ok", Data: "data"}
	if s.Status != "ok" {
		t.Errorf("expected 'ok', got '%s'", s.Status)
	}
}