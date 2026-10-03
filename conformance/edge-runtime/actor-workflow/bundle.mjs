// Unpublished executable input. A consumer Host, not this module, supplies
// Actor context, SQL, alarms, sockets, Workflow steps, and isolation.
export class OrdinaryActor {
  constructor(context, env) {
    this.context = context;
    this.env = env;
  }

  fetch(_request, _turn) {
    return Response.json({ id: this.context.id, marker: this.env.MARKER });
  }

  alarm(_turn) {}
  socketMessage(_socket, _data, _turn) {}
  socketClose(_socket, _event, _turn) {}
  socketError(_socket, _event, _turn) {}
}

const inheritedHandlers = {
  fetch() {
    return new Response("inherited");
  },
  alarm() {},
  socketMessage() {},
  socketClose() {},
  socketError() {},
};

export class DeepInheritedActor {
  constructor(context, env) {
    this.context = context;
    this.env = env;
  }
}

let inheritedPrototype = inheritedHandlers;
for (let depth = 0; depth < 129; depth += 1) {
  inheritedPrototype = Object.create(inheritedPrototype);
}
Object.setPrototypeOf(DeepInheritedActor.prototype, inheritedPrototype);

export class SignalWorkflow {
  constructor(env) {
    this.env = env;
  }

  async run(event, step) {
    const recorded = await step.do("record", async () => ({
      id: event.params.id,
      marker: this.env.MARKER,
    }));
    const signal = await step.waitForEvent("approval", {
      type: "approved",
      timeoutSeconds: 60,
    });
    return { id: recorded.id, approved: signal?.approved === true };
  }
}

// Proxy fixtures are factories: importing this bundle must never run a
// potentially non-returning tenant trap in the corpus reader's process.
function fourHandlerActor() {
  return class Actor {
    fetch() {
      return new Response("ok");
    }
    alarm() {}
    socketMessage() {}
    socketClose() {}
  };
}

export function cyclicProxyActorModule() {
  const Actor = fourHandlerActor();
  let cycle;
  cycle = new Proxy({}, { getPrototypeOf: () => cycle });
  Object.setPrototypeOf(Actor.prototype, cycle);
  return { Actor };
}

export function changingProxyActorModule() {
  const Actor = fourHandlerActor();
  let reads = 0;
  const changing = new Proxy(
    {},
    {
      getPrototypeOf() {
        reads += 1;
        return reads === 1 ? null : { socketError() {} };
      },
    },
  );
  Object.setPrototypeOf(Actor.prototype, changing);
  return { Actor };
}

export function nonReturningProxyActorModule() {
  const Actor = fourHandlerActor();
  const blocked = new Proxy(
    {},
    {
      getPrototypeOf() {
        for (;;) {
          /* must run only in a killable inspection child */
        }
      },
    },
  );
  Object.setPrototypeOf(Actor.prototype, blocked);
  return { Actor };
}

export default {
  fetch() {
    return new Response("worker");
  },
  scheduled() {},
  queue() {},
};
