export namespace core {
	
	export class CheckResult {
	    ok: boolean;
	    ms: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.ms = source["ms"];
	        this.message = source["message"];
	    }
	}
	export class ConnectResult {
	    count: number;
	    ok: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.ok = source["ok"];
	        this.message = source["message"];
	    }
	}
	export class Snapshot {
	    envs: model.Environment[];
	    current: string;
	    theme: string;
	    lang: string;
	    allowLan: boolean;
	    running: boolean;
	    localIp: string;
	    reqCount: number;
	    avgMs: number;
	    lastSync: string;
	    logs: model.LogEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.envs = this.convertValues(source["envs"], model.Environment);
	        this.current = source["current"];
	        this.theme = source["theme"];
	        this.lang = source["lang"];
	        this.allowLan = source["allowLan"];
	        this.running = source["running"];
	        this.localIp = source["localIp"];
	        this.reqCount = source["reqCount"];
	        this.avgMs = source["avgMs"];
	        this.lastSync = source["lastSync"];
	        this.logs = this.convertValues(source["logs"], model.LogEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace model {
	
	export class Rule {
	    svc: string;
	    path: string;
	    dest: string;
	    prio: number;
	    remark?: string;
	    on: boolean;
	    stripPrefix?: string;
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.svc = source["svc"];
	        this.path = source["path"];
	        this.dest = source["dest"];
	        this.prio = source["prio"];
	        this.remark = source["remark"];
	        this.on = source["on"];
	        this.stripPrefix = source["stripPrefix"];
	    }
	}
	export class Service {
	    name: string;
	    src: string;
	    on: boolean;
	    online: boolean;
	    port: number;
	    addr?: string;
	    remark?: string;
	    prefix?: string;
	
	    static createFrom(source: any = {}) {
	        return new Service(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.src = source["src"];
	        this.on = source["on"];
	        this.online = source["online"];
	        this.port = source["port"];
	        this.addr = source["addr"];
	        this.remark = source["remark"];
	        this.prefix = source["prefix"];
	    }
	}
	export class Registry {
	    type: string;
	    addr: string;
	    ns?: string;
	    group?: string;
	    nacosVersion?: string;
	    nacosUser?: string;
	    nacosPass?: string;
	    user?: string;
	    pass?: string;
	    dc?: string;
	    token?: string;
	    ok: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Registry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.addr = source["addr"];
	        this.ns = source["ns"];
	        this.group = source["group"];
	        this.nacosVersion = source["nacosVersion"];
	        this.nacosUser = source["nacosUser"];
	        this.nacosPass = source["nacosPass"];
	        this.user = source["user"];
	        this.pass = source["pass"];
	        this.dc = source["dc"];
	        this.token = source["token"];
	        this.ok = source["ok"];
	    }
	}
	export class Environment {
	    name: string;
	    port: number;
	    reg?: Registry;
	    gw: string;
	    gwOk: boolean;
	    defaultTarget: string;
	    defaultAddr: string;
	    probe: number;
	    services: Service[];
	    rules: Rule[];
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Environment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.port = source["port"];
	        this.reg = this.convertValues(source["reg"], Registry);
	        this.gw = source["gw"];
	        this.gwOk = source["gwOk"];
	        this.defaultTarget = source["defaultTarget"];
	        this.defaultAddr = source["defaultAddr"];
	        this.probe = source["probe"];
	        this.services = this.convertValues(source["services"], Service);
	        this.rules = this.convertValues(source["rules"], Rule);
	        this.running = source["running"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LogEntry {
	    t: string;
	    m: string;
	    p: string;
	    raw: string;
	    final: string;
	    target: string;
	    s: string;
	    dec: string;
	    c: number;
	    ms: number;
	    sname: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.t = source["t"];
	        this.m = source["m"];
	        this.p = source["p"];
	        this.raw = source["raw"];
	        this.final = source["final"];
	        this.target = source["target"];
	        this.s = source["s"];
	        this.dec = source["dec"];
	        this.c = source["c"];
	        this.ms = source["ms"];
	        this.sname = source["sname"];
	    }
	}
	
	

}

export namespace version {
	
	export class CheckUpdateResult {
	    hasUpdate: boolean;
	    version: string;
	    note: string;
	    download: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckUpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUpdate = source["hasUpdate"];
	        this.version = source["version"];
	        this.note = source["note"];
	        this.download = source["download"];
	        this.message = source["message"];
	    }
	}

}

