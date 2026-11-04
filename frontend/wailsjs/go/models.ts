export namespace main {
	
	export class Booking {
	    id: string;
	    projectId: string;
	    start: string;
	    end: string;
	    notes: string;
	    exported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Booking(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.notes = source["notes"];
	        this.exported = source["exported"];
	    }
	}
	export class ExportOptions {
	    markExported: boolean;
	    excludeExported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.markExported = source["markExported"];
	        this.excludeExported = source["excludeExported"];
	    }
	}
	export class Timer {
	    projectId: string;
	    start: string;
	
	    static createFrom(source: any = {}) {
	        return new Timer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectId = source["projectId"];
	        this.start = source["start"];
	    }
	}
	export class State {
	    projects: tracker.Project[];
	    bookings: Booking[];
	    active?: Timer;
	    pausedProjectId: string;
	    settings: tracker.Settings;
	    propertyDefinitions: tracker.PropertyDefinition[];
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projects = this.convertValues(source["projects"], tracker.Project);
	        this.bookings = this.convertValues(source["bookings"], Booking);
	        this.active = this.convertValues(source["active"], Timer);
	        this.pausedProjectId = source["pausedProjectId"];
	        this.settings = this.convertValues(source["settings"], tracker.Settings);
	        this.propertyDefinitions = this.convertValues(source["propertyDefinitions"], tracker.PropertyDefinition);
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
	export class TemporaryAssignment {
	    timer: Timer;
	    action: string;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new TemporaryAssignment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timer = this.convertValues(source["timer"], Timer);
	        this.action = source["action"];
	        this.id = source["id"];
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

export namespace tracker {
	
	export class Project {
	    id: string;
	    number: string;
	    name: string;
	    customer: string;
	    parentId: string;
	    color: string;
	    icon?: string;
	    favorite: boolean;
	    archived: boolean;
	    properties: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.number = source["number"];
	        this.name = source["name"];
	        this.customer = source["customer"];
	        this.parentId = source["parentId"];
	        this.color = source["color"];
	        this.icon = source["icon"];
	        this.favorite = source["favorite"];
	        this.archived = source["archived"];
	        this.properties = source["properties"];
	    }
	}
	export class PropertyDefinition {
	    id: string;
	    name: string;
	    type: string;
	    options: string[];
	
	    static createFrom(source: any = {}) {
	        return new PropertyDefinition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.options = source["options"];
	    }
	}
	export class ReminderSettings {
	    runningEnabled: boolean;
	    runningMinutes: number;
	    idleEnabled: boolean;
	    idleMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new ReminderSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runningEnabled = source["runningEnabled"];
	        this.runningMinutes = source["runningMinutes"];
	        this.idleEnabled = source["idleEnabled"];
	        this.idleMinutes = source["idleMinutes"];
	    }
	}
	export class Settings {
	    requireTemporaryProject: boolean;
	    theme: string;
	    reminders: ReminderSettings;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requireTemporaryProject = source["requireTemporaryProject"];
	        this.theme = source["theme"];
	        this.reminders = this.convertValues(source["reminders"], ReminderSettings);
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

