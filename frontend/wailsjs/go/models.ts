export namespace app {

	export class Product {
	    ID: number;
	    Name: string;
	    Multizone: boolean;
	    Matrix: boolean;
	    Chain: boolean;

	    static createFrom(source: any = {}) {
	        return new Product(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Multizone = source["Multizone"];
	        this.Matrix = source["Matrix"];
	        this.Chain = source["Chain"];
	    }
	}
	export class View {
	    Transport: lan.TransportStats;
	    Devices: lan.Snapshot[];
	    Recent: lan.Activity[];
	    Listening: string;
	    Error: string;
	    Interfaces: string[];

	    static createFrom(source: any = {}) {
	        return new View(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Transport = this.convertValues(source["Transport"], lan.TransportStats);
	        this.Devices = this.convertValues(source["Devices"], lan.Snapshot);
	        this.Recent = this.convertValues(source["Recent"], lan.Activity);
	        this.Listening = source["Listening"];
	        this.Error = source["Error"];
	        this.Interfaces = source["Interfaces"];
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

export namespace config {

	export class Definition {
	    Serial: string;
	    Label: string;
	    Product: number;
	    Enabled: boolean;
	    Zones: number;
	    Width: number;
	    Height: number;
	    Chains: number;
	    Orientations: number[];

	    static createFrom(source: any = {}) {
	        return new Definition(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Serial = source["Serial"];
	        this.Label = source["Label"];
	        this.Product = source["Product"];
	        this.Enabled = source["Enabled"];
	        this.Zones = source["Zones"];
	        this.Width = source["Width"];
	        this.Height = source["Height"];
	        this.Chains = source["Chains"];
	        this.Orientations = source["Orientations"];
	    }
	}

}

export namespace device {

	export class Color {
	    Hue: number;
	    Saturation: number;
	    Brightness: number;
	    Kelvin: number;

	    static createFrom(source: any = {}) {
	        return new Color(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Hue = source["Hue"];
	        this.Saturation = source["Saturation"];
	        this.Brightness = source["Brightness"];
	        this.Kelvin = source["Kelvin"];
	    }
	}
	export class MatrixRow {
	    Cols: number;
	    Offset: number;
	    HiddenCols: number[];

	    static createFrom(source: any = {}) {
	        return new MatrixRow(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Cols = source["Cols"];
	        this.Offset = source["Offset"];
	        this.HiddenCols = source["HiddenCols"];
	    }
	}
	export class Rect {
	    X: number;
	    Y: number;
	    Width: number;
	    Height: number;

	    static createFrom(source: any = {}) {
	        return new Rect(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.X = source["X"];
	        this.Y = source["Y"];
	        this.Width = source["Width"];
	        this.Height = source["Height"];
	    }
	}
	export class MatrixChain {
	    Index: number;
	    Bounds: Rect;
	    SendWidth: number;
	    Rows: MatrixRow[];
	    Orientation: number;

	    static createFrom(source: any = {}) {
	        return new MatrixChain(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Index = source["Index"];
	        this.Bounds = this.convertValues(source["Bounds"], Rect);
	        this.SendWidth = source["SendWidth"];
	        this.Rows = this.convertValues(source["Rows"], MatrixRow);
	        this.Orientation = source["Orientation"];
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

	export class MatrixSurface {
	    Chains: MatrixChain[];

	    static createFrom(source: any = {}) {
	        return new MatrixSurface(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Chains = this.convertValues(source["Chains"], MatrixChain);
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

	export class Surface {
	    LightType: number;
	    Width: number;
	    Height: number;
	    Zones: number;
	    Matrix?: MatrixSurface;

	    static createFrom(source: any = {}) {
	        return new Surface(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.LightType = source["LightType"];
	        this.Width = source["Width"];
	        this.Height = source["Height"];
	        this.Zones = source["Zones"];
	        this.Matrix = this.convertValues(source["Matrix"], MatrixSurface);
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

export namespace lan {

	export class Activity {
	    Direction: string;
	    Peer: string;
	    Source: number;
	    Sequence: number;
	    Replies: number;
	    Error: string;
	    // Go type: time
	    At: any;
	    Target: string;
	    Type: number;
	    TypeName: string;
	    Label: string;
	    Applied: boolean;

	    static createFrom(source: any = {}) {
	        return new Activity(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Direction = source["Direction"];
	        this.Peer = source["Peer"];
	        this.Source = source["Source"];
	        this.Sequence = source["Sequence"];
	        this.Replies = source["Replies"];
	        this.Error = source["Error"];
	        this.At = this.convertValues(source["At"], null);
	        this.Target = source["Target"];
	        this.Type = source["Type"];
	        this.TypeName = source["TypeName"];
	        this.Label = source["Label"];
	        this.Applied = source["Applied"];
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
	export class Snapshot {
	    Serial: string;
	    Label: string;
	    Product: number;
	    Model: string;
	    Kind: string;
	    Enabled: boolean;
	    Power: number;
	    Surface: device.Surface;
	    Colors: device.Color[];
	    Active: boolean;

	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Serial = source["Serial"];
	        this.Label = source["Label"];
	        this.Product = source["Product"];
	        this.Model = source["Model"];
	        this.Kind = source["Kind"];
	        this.Enabled = source["Enabled"];
	        this.Power = source["Power"];
	        this.Surface = this.convertValues(source["Surface"], device.Surface);
	        this.Colors = this.convertValues(source["Colors"], device.Color);
	        this.Active = source["Active"];
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
	export class TransportStats {
	    Received: number;
	    Decoded: number;
	    Filtered: number;
	    Invalid: number;
	    Replies: number;
	    SendErrors: number;
	    LastPeer: string;
	    LastError: string;

	    static createFrom(source: any = {}) {
	        return new TransportStats(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Received = source["Received"];
	        this.Decoded = source["Decoded"];
	        this.Filtered = source["Filtered"];
	        this.Invalid = source["Invalid"];
	        this.Replies = source["Replies"];
	        this.SendErrors = source["SendErrors"];
	        this.LastPeer = source["LastPeer"];
	        this.LastError = source["LastError"];
	    }
	}

}

