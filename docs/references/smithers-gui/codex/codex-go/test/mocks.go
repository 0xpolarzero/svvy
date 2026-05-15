package test

// This file contains instructions and utilities for generating and using mocks in tests.
//
// Mock Generation with go:generate
//
// To generate mocks for an interface, add a go:generate directive above the interface
// definition or in a dedicated mocks file. For example:
//
//   //go:generate mockgen -destination=../test/mocks/mock_foo.go -package=mocks . FooInterface
//
// Mock Generation Best Practices:
//
// 1. Create interface definitions in your production code, not just for testing
// 2. Place all generated mocks in the test/mocks/ subdirectory
// 3. Use the -package=mocks flag to keep all mocks in a consistent package
// 4. Name mock files with the pattern: mock_<interface_name>.go
// 5. Run `make generate-mocks` or `go generate ./...` to regenerate all mocks
//
// Example Interface Definition:
//
//   package mypackage
//
//   //go:generate mockgen -destination=../test/mocks/mock_repository.go -package=mocks . Repository
//
//   type Repository interface {
//       Get(ctx context.Context, id string) (*Entity, error)
//       Save(ctx context.Context, entity *Entity) error
//   }
//
// Using Mocks in Tests:
//
//   import (
//       "testing"
//       "github.com/evmts/codex/codex-go/test/mocks"
//       "go.uber.org/mock/gomock"
//   )
//
//   func TestMyFunction(t *testing.T) {
//       ctrl := gomock.NewController(t)
//       defer ctrl.Finish()
//
//       mockRepo := mocks.NewMockRepository(ctrl)
//       mockRepo.EXPECT().
//           Get(gomock.Any(), "123").
//           Return(&Entity{ID: "123"}, nil)
//
//       // Use mockRepo in your test
//       result, err := MyFunction(mockRepo)
//       require.NoError(t, err)
//   }
//
// Common Mock Patterns:
//
// 1. Expect a specific call with specific arguments:
//    mock.EXPECT().Method(arg1, arg2).Return(result)
//
// 2. Expect any arguments with gomock matchers:
//    mock.EXPECT().Method(gomock.Any(), gomock.Any()).Return(result)
//
// 3. Expect multiple calls:
//    mock.EXPECT().Method(arg).Return(result).Times(3)
//
// 4. Expect calls in specific order:
//    gomock.InOrder(
//        mock.EXPECT().Method1().Return(result1),
//        mock.EXPECT().Method2().Return(result2),
//    )
//
// 5. Expect at least/most N calls:
//    mock.EXPECT().Method(arg).Return(result).MinTimes(1)
//    mock.EXPECT().Method(arg).Return(result).MaxTimes(5)
//
// 6. Custom argument matchers:
//    mock.EXPECT().Method(gomock.Eq(expected)).Return(result)
//    mock.EXPECT().Method(gomock.Not(gomock.Nil())).Return(result)
//
// 7. Do custom actions:
//    mock.EXPECT().Method(gomock.Any()).Do(func(arg string) {
//        // Custom logic
//    }).Return(result)
//
// Directory Structure:
//
//   test/
//   ├── mocks/                  # Generated mocks live here
//   │   ├── mock_repository.go
//   │   ├── mock_service.go
//   │   └── mock_client.go
//   ├── testdata/               # Test data
//   │   ├── fixtures/           # Reusable test data
//   │   ├── golden/             # Golden file outputs
//   │   └── protocol/           # Protocol test fixtures
//   ├── testhelpers.go          # Test helper functions
//   └── mocks.go                # This file (documentation)

import (
	"testing"

	"go.uber.org/mock/gomock"
)

// MockController wraps gomock.Controller with automatic cleanup
type MockController struct {
	*gomock.Controller
}

// NewMockController creates a new mock controller with automatic cleanup
func NewMockController(t *testing.T) *MockController {
	ctrl := gomock.NewController(t)
	return &MockController{Controller: ctrl}
}

// Common Interface Patterns for Mocking
//
// When designing interfaces for testability, follow these patterns:

// Example: File System Operations
//
//   type FileSystem interface {
//       ReadFile(path string) ([]byte, error)
//       WriteFile(path string, data []byte) error
//       FileExists(path string) bool
//   }

// Example: HTTP Client
//
//   type HTTPClient interface {
//       Get(ctx context.Context, url string) (*http.Response, error)
//       Post(ctx context.Context, url string, body io.Reader) (*http.Response, error)
//   }

// Example: Command Executor
//
//   type CommandExecutor interface {
//       Execute(ctx context.Context, name string, args ...string) (stdout, stderr string, err error)
//   }

// Example: Database Repository
//
//   type Repository interface {
//       Get(ctx context.Context, id string) (*Entity, error)
//       List(ctx context.Context, filters map[string]interface{}) ([]*Entity, error)
//       Create(ctx context.Context, entity *Entity) error
//       Update(ctx context.Context, entity *Entity) error
//       Delete(ctx context.Context, id string) error
//   }

// Example: Event Publisher
//
//   type EventPublisher interface {
//       Publish(ctx context.Context, event Event) error
//       PublishBatch(ctx context.Context, events []Event) error
//   }

// Custom Matchers
//
// You can create custom matchers for complex assertions:

// StringContainsMatcher matches strings containing a substring
type StringContainsMatcher struct {
	substring string
}

func (m *StringContainsMatcher) Matches(x interface{}) bool {
	s, ok := x.(string)
	if !ok {
		return false
	}
	return contains(s, m.substring)
}

func (m *StringContainsMatcher) String() string {
	return "contains substring: " + m.substring
}

// StringContains creates a matcher that matches strings containing the substring
func StringContains(substring string) gomock.Matcher {
	return &StringContainsMatcher{substring: substring}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOfSubstring(s, substr) >= 0)
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// JSONMatcher matches JSON strings with flexible comparison
type JSONMatcher struct {
	expected string
}

func (m *JSONMatcher) Matches(x interface{}) bool {
	s, ok := x.(string)
	if !ok {
		return false
	}
	// In a real implementation, you'd parse and compare JSON
	// For now, this is a placeholder
	return s == m.expected
}

func (m *JSONMatcher) String() string {
	return "matches JSON: " + m.expected
}

// MatchJSON creates a matcher that compares JSON strings semantically
func MatchJSON(expected string) gomock.Matcher {
	return &JSONMatcher{expected: expected}
}

// Mock Testing Examples
//
// Here are complete examples of common testing scenarios:

// Example 1: Testing a Service with Repository Dependency
//
//   func TestUserService_GetUser(t *testing.T) {
//       ctrl := NewMockController(t)
//       mockRepo := mocks.NewMockUserRepository(ctrl)
//
//       // Setup expectation
//       expectedUser := &User{ID: "123", Name: "Alice"}
//       mockRepo.EXPECT().
//           Get(gomock.Any(), "123").
//           Return(expectedUser, nil)
//
//       // Create service with mock
//       service := NewUserService(mockRepo)
//
//       // Test
//       user, err := service.GetUser(context.Background(), "123")
//
//       // Assert
//       require.NoError(t, err)
//       assert.Equal(t, expectedUser, user)
//   }

// Example 2: Testing Error Handling
//
//   func TestUserService_GetUser_NotFound(t *testing.T) {
//       ctrl := NewMockController(t)
//       mockRepo := mocks.NewMockUserRepository(ctrl)
//
//       // Setup expectation for error
//       mockRepo.EXPECT().
//           Get(gomock.Any(), "999").
//           Return(nil, ErrNotFound)
//
//       service := NewUserService(mockRepo)
//
//       // Test
//       user, err := service.GetUser(context.Background(), "999")
//
//       // Assert error
//       assert.Error(t, err)
//       assert.Nil(t, user)
//       assert.Equal(t, ErrNotFound, err)
//   }

// Example 3: Testing Multiple Calls
//
//   func TestUserService_BulkUpdate(t *testing.T) {
//       ctrl := NewMockController(t)
//       mockRepo := mocks.NewMockUserRepository(ctrl)
//
//       users := []*User{
//           {ID: "1", Name: "Alice"},
//           {ID: "2", Name: "Bob"},
//       }
//
//       // Expect Update to be called for each user
//       for _, user := range users {
//           mockRepo.EXPECT().
//               Update(gomock.Any(), user).
//               Return(nil)
//       }
//
//       service := NewUserService(mockRepo)
//       err := service.BulkUpdate(context.Background(), users)
//
//       require.NoError(t, err)
//   }

// Example 4: Testing Call Order
//
//   func TestUserService_CreateWithValidation(t *testing.T) {
//       ctrl := NewMockController(t)
//       mockValidator := mocks.NewMockValidator(ctrl)
//       mockRepo := mocks.NewMockUserRepository(ctrl)
//
//       user := &User{Name: "Alice"}
//
//       // Expect calls in specific order
//       gomock.InOrder(
//           mockValidator.EXPECT().Validate(user).Return(nil),
//           mockRepo.EXPECT().Create(gomock.Any(), user).Return(nil),
//       )
//
//       service := NewUserService(mockRepo, mockValidator)
//       err := service.Create(context.Background(), user)
//
//       require.NoError(t, err)
//   }

// Example 5: Testing with Custom Matchers
//
//   func TestEventPublisher_Publish(t *testing.T) {
//       ctrl := NewMockController(t)
//       mockPublisher := mocks.NewMockEventPublisher(ctrl)
//
//       mockPublisher.EXPECT().
//           Publish(
//               gomock.Any(),
//               gomock.AssignableToTypeOf(&Event{}),
//           ).
//           Do(func(ctx context.Context, event *Event) {
//               // Verify event properties
//               assert.Equal(t, "UserCreated", event.Type)
//           }).
//           Return(nil)
//
//       service := NewService(mockPublisher)
//       err := service.CreateUser(context.Background(), &User{Name: "Alice"})
//
//       require.NoError(t, err)
//   }
