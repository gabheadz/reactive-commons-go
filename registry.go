package rcommons

type Registry struct {
	eventHandlers   map[string]EventHandler
	commandHandlers map[string]CommandHandler
	queryHandlers   map[string]QueryHandler
}

func NewRegistry() Registry {
	return Registry{
		eventHandlers:   make(map[string]EventHandler),
		commandHandlers: make(map[string]CommandHandler),
		queryHandlers:   make(map[string]QueryHandler),
	}
}

func (r *Registry) ListenEvent(eventName string, handler EventHandler) {
	r.eventHandlers[eventName] = handler
}

// ListenEventTyped registers a type-safe event handler
func ListenEventTyped[T any](r Registry, eventName string, handler func(event T) error) {
	r.eventHandlers[eventName] = EventHandlerFunc[T](handler).ToEventHandler()
}

func (r *Registry) ListenEvents(handlers map[string]EventHandler) {
	for eventName, handler := range handlers {
		r.ListenEvent(eventName, handler)
	}
}

func (r *Registry) HandleCommand(commandName string, handler CommandHandler) {
	r.commandHandlers[commandName] = handler
}

func (r *Registry) HandleCommands(handlers map[string]CommandHandler) {
	for commandName, handler := range handlers {
		r.HandleCommand(commandName, handler)
	}
}

func (r *Registry) ServeQuery(resource string, handler QueryHandler) {
	r.queryHandlers[resource] = handler
}

func (r *Registry) ServeQueries(handlers map[string]QueryHandler) {
	for resource, handler := range handlers {
		r.ServeQuery(resource, handler)
	}
}

func (r *Registry) HasEventHandler(eventName string) bool {
	_, exists := r.eventHandlers[eventName]
	return exists
}

func (r *Registry) HasCommandHandler(commandName string) bool {
	_, exists := r.commandHandlers[commandName]
	return exists
}

func (r *Registry) HasQueryHandler(resource string) bool {
	_, exists := r.queryHandlers[resource]
	return exists
}

func (r *Registry) GetEventHandler(eventName string) (EventHandler, bool) {
	h, exists := r.eventHandlers[eventName]
	return h, exists
}

func (r *Registry) GetCommandHandler(commandName string) (CommandHandler, bool) {
	h, exists := r.commandHandlers[commandName]
	return h, exists
}

func (r *Registry) GetQueryHandler(resource string) (QueryHandler, bool) {
	h, exists := r.queryHandlers[resource]
	return h, exists
}

func (r *Registry) GetEventHandlers() map[string]EventHandler {
	handlers := make(map[string]EventHandler, len(r.eventHandlers))
	for k, v := range r.eventHandlers {
		handlers[k] = v
	}
	return handlers
}

func (r *Registry) GetCommandHandlers() map[string]CommandHandler {
	handlers := make(map[string]CommandHandler, len(r.commandHandlers))
	for k, v := range r.commandHandlers {
		handlers[k] = v
	}
	return handlers
}

func (r *Registry) GetQueryHandlers() map[string]QueryHandler {
	handlers := make(map[string]QueryHandler, len(r.queryHandlers))
	for k, v := range r.queryHandlers {
		handlers[k] = v
	}
	return handlers
}

func (r *Registry) EventHandlersCount() int {
	return len(r.eventHandlers)
}

func (r *Registry) CommandHandlersCount() int {
	return len(r.commandHandlers)
}

func (r *Registry) QueryHandlersCount() int {
	return len(r.queryHandlers)
}

func (r *Registry) Clear() {
	r.eventHandlers = make(map[string]EventHandler)
	r.commandHandlers = make(map[string]CommandHandler)
	r.queryHandlers = make(map[string]QueryHandler)
}
