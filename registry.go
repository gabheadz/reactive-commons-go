package rcommons

type Registry struct {
	EventHandlers   map[string]EventHandler
	CommandHandlers map[string]CommandHandler
	QueryHandlers   map[string]QueryHandler
}

func NewRegistry() *Registry {
	return &Registry{
		EventHandlers:   make(map[string]EventHandler),
		CommandHandlers: make(map[string]CommandHandler),
		QueryHandlers:   make(map[string]QueryHandler),
	}
}

func (r *Registry) ListenEvent(eventName string, handler EventHandler) {
	r.EventHandlers[eventName] = handler
}

// ListenEventTyped registers a type-safe event handler
func ListenEventTyped[T any](r *Registry, eventName string, handler func(event T) error) {
	r.EventHandlers[eventName] = EventHandlerFunc[T](handler).ToEventHandler()
}

func (r *Registry) ListenEvents(handlers map[string]EventHandler) {
	for eventName, handler := range handlers {
		r.ListenEvent(eventName, handler)
	}
}

func (r *Registry) HandleCommand(commandName string, handler CommandHandler) {
	r.CommandHandlers[commandName] = handler
}

func (r *Registry) HandleCommands(handlers map[string]CommandHandler) {
	for commandName, handler := range handlers {
		r.HandleCommand(commandName, handler)
	}
}

func (r *Registry) ServeQuery(resource string, handler QueryHandler) {
	r.QueryHandlers[resource] = handler
}

func (r *Registry) ServeQueries(handlers map[string]QueryHandler) {
	for resource, handler := range handlers {
		r.ServeQuery(resource, handler)
	}
}

func (r *Registry) HasEventHandler(eventName string) bool {
	_, exists := r.EventHandlers[eventName]
	return exists
}

func (r *Registry) Clear() {
	r.EventHandlers = make(map[string]EventHandler)
	r.CommandHandlers = make(map[string]CommandHandler)
	r.QueryHandlers = make(map[string]QueryHandler)	
}
