package rcommons

import (
	"fmt"
	"log"
	"sync"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
	"github.com/google/uuid"
)

const domainEvents = "domainEvents"
const domainEventsSuffix = "subsEvents"

const directExchange = "directMessages"
const directCommandsSuffix = "direct"

const globalExchange = "globalReply"
const directQuerySuffix = "query"
const globalRepliesSuffix = "replies"

const ChannelForEvents string = "ChannelForEvents"
const ChannelForCommands string = "ChannelForCommands"
const ChannelForQueries string = "ChannelForQueries"
const ChannelForReplies string = "ChannelForReplies"

type ReactiveCommons struct {
	Domain          DomainDefinition
	connections     map[string]*rabbit.RabbitClient
	registry        Registry
	startOnce       sync.Once
	eventPublisher  *rabbitEventPublisher
	eventListener   *rabbitEventListener
	commandSender   *rabbitCommandSender
	commandReceiver *rabbitCommandReceiver
	queryClient     *rabbitQueryClient
	queryServer     *rabbitQueryServer
	topologyManager *rabbitTopologyManager
}

func NewReactiveCommons(domainCfg DomainDefinition) *ReactiveCommons {
	if domainCfg.Name == "" {
		log.Panic("invalid domain name")
	}

	connections := make(map[string]*rabbit.RabbitClient)

	// Setup default names for events, if not provided
	if domainCfg.DomainEventsExchange == "" {
		domainCfg.DomainEventsExchange = domainEvents
	}
	if domainCfg.DomainEventsSuffix == "" {
		domainCfg.DomainEventsSuffix = domainEventsSuffix
	}
	// for commands
	if domainCfg.DirectExchange == "" {
		domainCfg.DirectExchange = directExchange
	}
	if domainCfg.DirectCommandsSuffix == "" {
		domainCfg.DirectCommandsSuffix = directCommandsSuffix
	}
	// for queries
	if domainCfg.DirectQuerySuffix == "" {
		domainCfg.DirectQuerySuffix = directQuerySuffix
	}
	if domainCfg.GlobalExchange == "" {
		domainCfg.GlobalExchange = globalExchange
	}
	if domainCfg.GlobalRepliesSuffix == "" {
		domainCfg.GlobalRepliesSuffix = globalRepliesSuffix
	}
	domainCfg.globalBindID = uuid.New().String()
	domainCfg.globalRepliesID = calculateQueueName(domainCfg.Name, domainCfg.GlobalRepliesSuffix, true)

	rc := &ReactiveCommons{
		Domain:      domainCfg,
		connections: connections,
	}

	return rc
}

func (rc *ReactiveCommons) Start(registry Registry) {
	// this can be called only once, subsequent calls will be ignored
	rc.startOnce.Do(func() {
		err := rc.checkSwitches(registry)
		if err != nil {
			log.Panicf("Failed to start ReactiveCommons: %v", err)
		}

		rc.registry = registry

		// set up connections
		connAddr := connectionString(rc.Domain.ConnectionConfig)
		cli, _ := rabbit.NewRabbitClient(rc.Domain.Name, connAddr)
		err = cli.Connect()
		if err != nil {
			log.Panicf("Failed to connect to RabbitMQ: %v", err)
		}
		rc.connections["sending"] = cli

		//check if separate connection is required for receiving, if so create and connect
		if rc.Domain.ConnectionConfig.SeparateConnections {
			cli2, _ := rabbit.NewRabbitClient(rc.Domain.Name, connAddr)
			err = cli2.Connect()
			if err != nil {
				log.Panicf("Failed to connect to RabbitMQ: %v", err)
			}
			rc.connections["receiving"] = cli2
		} else {
			rc.connections["receiving"] = cli
		}

		// Initialize topology manager
		rc.topologyManager = newTopologyManager(&rc.Domain)

		if rc.Domain.UseDomainEvents {
			err = rc.topologyManager.setupDomainEvents(rc.connections["receiving"], rc.registry.GetEventHandlers())
			if err != nil {
				log.Printf("Failed to configure for domain events: %v", err)
				return
			}
			rc.eventListener = newEventListener(rc.connections["receiving"], &rc.Domain, &rc.registry)
			rc.eventListener.startListeningEvents()

			rc.eventPublisher = newEventPublisher(rc.connections["sending"], &rc.Domain)
		}

		if rc.Domain.UseDirectCommands || rc.Domain.UseDirectQueries {
			err := rc.topologyManager.setupDirectCommands(rc.connections["receiving"])
			if err != nil {
				log.Printf("Failed to configure for direct commands: %v", err)
				return
			}
			rc.commandReceiver = newCommandReceiver(rc.connections["receiving"], &rc.Domain, &rc.registry)
			rc.commandReceiver.startListeningCommands()
			rc.commandSender = newCommandSender(rc.connections["sending"], &rc.Domain)

			if rc.Domain.UseDirectQueries {
				err := rc.topologyManager.setupAsyncQueries(rc.connections["receiving"])
				if err != nil {
					log.Printf("Failed to configure for async queries: %v", err)
					return
				}
				rc.queryServer = newQueryServer(rc.connections["receiving"], &rc.Domain, &rc.registry)
				rc.queryServer.serveQueries()
				rc.queryClient = newQueryClient(rc.connections["sending"], rc.connections["receiving"], &rc.Domain)
			}
		}
	})
}

// -------------------------------
// Events - Delegate to components
// -------------------------------

func (rc *ReactiveCommons) EmitEvent(event DomainEvent[any], opts EventOptions) error {
	if rc.eventPublisher == nil {
		return fmt.Errorf("event publisher is not initialized; call Start first")
	}
	return rc.eventPublisher.emitEvent(event, opts)
}

// -------------------------------
// Commands - Delegate to components
// -------------------------------

func (rc *ReactiveCommons) SendCommand(command Command[any], opts CommandOptions) error {
	if rc.commandSender == nil {
		return fmt.Errorf("command sender is not initialized; call Start first")
	}
	return rc.commandSender.sendCommand(command, opts)
}

// -------------------------------
// Queries - Delegate to components
// -------------------------------

func (rc *ReactiveCommons) SendQueryRequest(request AsyncQuery[any], opts RequestReplyOptions) ([]byte, error) {
	if rc.queryClient == nil {
		return nil, fmt.Errorf("query client is not initialized; call Start first")
	}
	return rc.queryClient.sendQueryRequest(request, opts)
}

func (rc *ReactiveCommons) checkSwitches(registry Registry) error {

	if !rc.Domain.UseDomainEvents {
		rc.Domain.UseDomainEvents = registry.EventHandlersCount() > 0
	}
	if !rc.Domain.UseDirectQueries {
		rc.Domain.UseDirectQueries = registry.QueryHandlersCount() > 0
	}
	if !rc.Domain.UseDirectCommands {
		rc.Domain.UseDirectCommands = registry.CommandHandlersCount() > 0 || registry.QueryHandlersCount() > 0
	}

	if !rc.Domain.UseDomainEvents && !rc.Domain.UseDirectCommands && !rc.Domain.UseDirectQueries {
		return fmt.Errorf("ReactiveCommons not configured for [Events, Commands and/or Queries]")
	}

	return nil
}
