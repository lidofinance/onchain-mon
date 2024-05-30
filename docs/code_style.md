# Code Style

## Don't panic

* It's possible to  throw panic on app initialization stage (app or worker) (including `os.Exit` or `log.Fatal()` )
* Don't panic in different places (at all). Just see point #.1
* [effective-go#panic](https://golang.org/doc/effective_go.html#panic)

  ```plaintext
  ...real library functions should avoid panic. If the problem can be masked or worked around,
  it's always better to let things continue to run rather than taking down the whole program.
  One possible counterexample is during initialization: if the library truly cannot set itself up,
  it might be reasonable to panic, so to speak.
  ```

## Accept interfaces return *structures | structures

* web app != library
* accept interfaces - Just remember about solid, polymorphism, testing and testing technics. Those things became possible when
* return structures -
    1. web app
        * struct - when object is short-lived, light and placed on the stack (example: dto)
        * pointer - long-lived + variants с nullable
    2. library
        * interface - when it's a public contract (opentracing, logger, cache facade/decorator, etc)
    3. [Dave Cheney @davecheney](https://twitter.com/davecheney/status/942596109406322688):

    ```plaintext
    As with all rules, there are exceptions. But the rule of thumb of accept interfaces return structs is a good guideline I think.
    ```

## Return data from methods

* func someMethod(somearg) (*string, error)
* func someMethod(somearg) (*User, error)
* func someMethod(somearg) ([]User, error)
* func someMethod(somearg) (map[int]User, error)
  Thus, methods can return a pair (nil, error) or (object, nil)

  ```go
  obj, err := someMethod(somearg)
  if err != nil {
    // ...
  }

  if obj == nil {
    // ...
  }
  ```

* Do NOT return pointer/s to references structures, interface (map, slices, channels)

  ```go
    return *[]smt, *map[smt]smt  
  ```

## Description of types

* Create a custom type for arrays (On pull-request, you should explain to colleagues why you used this approach)
* Do not make a type under slice, except when implementing public contracts (example - sorting <https://golang.org/pkg/sort/#Interface>)

  Not recommended:

  ```go
  type Role struct {
      ID   int    `json:"id"`
      Name string `json:"name"`
  }

  type Roles []Role // <- Not recommended:. It's enough []Role

  var RoleList = Roles{
      Role{ROLE_BOSS, `boss`},
      Role{ROLE_SPECIALIST, `specialist`},
  }
  ```

## Comparison and branching

* When checking a string for [not]emptiness, it is recommended to compare it with an empty string literal
  ```go
  // Good
  if s != "" {
    // ...
  }

  if s == "" {
    // ...
  }
  ```
  ```go
  // Bad
  if len(s) > 0 {
    // ...
  }

  if len(s) == 0 {
    // ...
  }
  ```
  The compiler still leads to one form, but it is much clearer this way.
  and it is clearly visible that the variable contains a string.

* Check slices/maps for [not]emptiness by comparing against zero
  ```go
  // Good
  if len(arr) == 0 {
    // ...
  }

  if len(arr) > 0 {
    // ...
  }
  ```
  ```go
  // Bad
  if len(arr) < 1 {
    // ...
  }
  ```
  The `len()` function does not return negative values

* Try to avoid using `else` as it leads to extra branching
  and may confuse the reader.
  ```go
  // Good
  if err != nil {
      log.Printf("Oh no!")
      return err
  }
  
  do()
  
  return nil
  ```
  ```go
  // Bad
  if err != nil {
      log.Printf("Oh no!")
  } else {
      do()
  }
  
  return err
  ```
  It is advisable to strive to ensure that the code is read sequentially.
  and there was no need to jump eyes to different parts of the screen

## Working with DTOs at the input (DtoIn)

* It is not recommended to drag DtoIn across layers.
* If DtoIn is small, then we shift the scalars to the method parameters (userID, objectID, type)
* If DtoIn is large, then we create a domain structure, convert DtoIn and already use it in the method signature
* If the DtoIn contains fields that you do not want to copy, because they are big - you can save the pointer, but don't forget about side effects when modifying

  ```go
  type OperationDtoIn {
    UserID   int64
    UserName string
    TariffID int64
    IsDealer bool
    IsFree   bool
    BigField string
  }

  // The structure itself must be in the entity folder. For example in internal/pkg/user/entity/user.go
  // see project-layout
  type User struct {
    ID       int64
    Name     string
    TariffID int64
    IsDealer bool
    IsFree   bool
    BigField *string
  }

  func NewUser(dto *OperationDtoIn) *User {
    return &User{
      ID:       dto.UserId,
      Name:     dto.UserName,
      TariffID: dto.TariffID,
      IsDealer: dto.IsDealer,
      IsFree:   dto.IsFree,
      BigField: &dto.BigField,
    }
  }

  func (h *Handler) Handle() {
    ...
    u := NewUser(dtoIn)
    service.ProcessUser(u)
    ...
  }
  ```

## When to use a pointer receiver

* when the method changes the receiver (especially when the result of this change should be visible from the outside)
* when the receiver contains a structure field from the `sync` package (`Mutex`, `RWMutex`, `Once` etc.)
* when receiver is a large struct/array
* when receiver's fields/contents (struct, array, slice) are pointers
* in all other cases

## Constructors for Handler, Usecase, Repository, etc

* A structure that is needed to implement some interface, such as `http.Handler`, should be private.
  This way clients won't be tempted to use it in their code like `type myStruct struct{h *pkg.Handler}` or `h := &pkg.Handler{..}`.
  In the first example, the code is bad due to the connection with a specific implementation of the interface (for example, it will not be possible to lock the behavior),
  and in the second - because of the danger of getting an error or panic when changing the private fields of the structure.

  Также см. выше про принцип `return structs, accept interfaces`

    ```go
    // Bad
    type Handler struct{}
    func New() *Handler {
        return &Handler{}
    }

    // Good
    type handler struct{}
    func New() *handler {
          return &handler{}
    }
    ```

## Unit - testing

* **Test names:** are best left without spaces. In case of errors, the console replaces spaces with substrings and then it is more difficult to search for a broken test:

    ```go
        name: `success super duper test`, -> bad
        name: `success_super_duper_test`, -> good
    ```

* **Mocks:** using the approach from [docs](structure.md), where interfaces are located, on any package, you can put a command to generate them
  while the mocks themselves will be located in `internal/pkg/<some_domain>/mocks`

  ```go
  //go:generate ../../../bin/mockery --name Usecase
  type Usecase interface {
  	User(ctx context.Context, UserID int64) (*entity.ChatUser, error)
  	CntActiveItems(ctx context.Context, UserID int64) (*int64, error)
  	Balance(ctx context.Context, UserID int64) (real, bonus *float64, err error)
  }
  ```

  Результат будет в

  ```text
    |        └───pkg
    |        |    │   
    |        |    └───domain
    |        |           | 
    |        |           └───mocks (generates automatically by mockery or similar apps)
    |        |           |      Repository.go
    |        |           |      Usecase.go
    |        |           └───entity
    |        |           |     your_model_name.go
    |        |           | 
    |        |           └───usecase
    |        |           |      usecase.go 
    |        |           |      usecase_test.go 
    |        |           | 
    |        |           └───repository
    |        |           |      repository.go 
    |        |           |      repository_test.go 
    |        |           |
    |        |           | repository.go (interface for repository)
    |        |           | usecase.go    (interface for usecase)
  ```

* If you are developing a library. please don't use testing frameworks (like `testify`) to minimize dependencies.
* Tools for unit testing:
    * [golang/mockgen](https://github.com/golang/mock#running-mockgen) - utility for automatic generation of mocks
    * [vektra/mockery](https://github.com/vektra/mockery) - another utility for automatic generation of mocks
    * [DATA-DOG/go-sqlmock](https://github.com/DATA-DOG/go-sqlmock) - mocks for calling SQL queries
    * [stretchr/testify](https://github.com/stretchr/testify) - convenient framework for testing. Remember that it is very suitable for services

## Long-term support and code refactoring

* Leaving TODO in the code, always put a link to the task where this TODO is proposed to be performed:
  ```go
  // Bad:

  // TODO: Remote this method
  func someOldMethod() {
    // ...
  }

  // Bad:

  // TODO: Remove this method link for a task
  func someOldMethod() {
    // ...
  }
  ```
* When developing libraries, try to mark functionality before releasing a new major version,
  which will be removed or changed in a backwards incompatible way with Deprecated:
  ```go
  // Deprecated: Use method NewMethod instead OldMethod
  func OldMethod() {
    // ... <- add fallback for using NewMethod
  }

  func NewMethod() {
    // ...
  }
  ```
  This approach will give users time to update and allow them to move to a new major release.
  with less problems.

## Links

* [Go Wiki. CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments#receiver-type)
* [Ardan Labs. Methods, interfaces and embedded types](https://www.ardanlabs.com/blog/2014/05/methods-interfaces-and-embedded-types.html)