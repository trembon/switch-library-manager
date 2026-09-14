export namespace db {
	
	export class TitleAttributes {
	    id: string;
	    name?: string;
	    version?: string;
	    region?: string;
	    releaseDate?: number;
	    ParsedReleaseDate: string;
	    publisher?: string;
	    iconUrl?: string;
	    screenshots?: string[];
	    bannerUrl?: string;
	    description?: string;
	    size?: number;
	    isDemo?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TitleAttributes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.region = source["region"];
	        this.releaseDate = source["releaseDate"];
	        this.ParsedReleaseDate = source["ParsedReleaseDate"];
	        this.publisher = source["publisher"];
	        this.iconUrl = source["iconUrl"];
	        this.screenshots = source["screenshots"];
	        this.bannerUrl = source["bannerUrl"];
	        this.description = source["description"];
	        this.size = source["size"];
	        this.isDemo = source["isDemo"];
	    }
	}

}

export namespace main {
	
	export class LibraryTemplateData {
	    id: number;
	    name: string;
	    version: string;
	    dlc: string;
	    titleId: string;
	    path: string;
	    icon: string;
	    update: number;
	    region: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new LibraryTemplateData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.dlc = source["dlc"];
	        this.titleId = source["titleId"];
	        this.path = source["path"];
	        this.icon = source["icon"];
	        this.update = source["update"];
	        this.region = source["region"];
	        this.type = source["type"];
	    }
	}
	export class Pair {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Pair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class LocalLibraryData {
	    library_data: LibraryTemplateData[];
	    issues: Pair[];
	    num_files: number;
	
	    static createFrom(source: any = {}) {
	        return new LocalLibraryData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.library_data = this.convertValues(source["library_data"], LibraryTemplateData);
	        this.issues = this.convertValues(source["issues"], Pair);
	        this.num_files = source["num_files"];
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
	
	export class SwitchTitle {
	    name: string;
	    titleId: string;
	    icon: string;
	    region: string;
	    release_date: string;
	
	    static createFrom(source: any = {}) {
	        return new SwitchTitle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.titleId = source["titleId"];
	        this.icon = source["icon"];
	        this.region = source["region"];
	        this.release_date = source["release_date"];
	    }
	}

}

export namespace process {
	
	export class IncompleteTitle {
	    Attributes: db.TitleAttributes;
	    Meta?: switchfs.ContentMetaAttributes;
	    local_update: number;
	    latest_update: number;
	    latest_update_date: string;
	    missing_dlc: string[];
	
	    static createFrom(source: any = {}) {
	        return new IncompleteTitle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Attributes = this.convertValues(source["Attributes"], db.TitleAttributes);
	        this.Meta = this.convertValues(source["Meta"], switchfs.ContentMetaAttributes);
	        this.local_update = source["local_update"];
	        this.latest_update = source["latest_update"];
	        this.latest_update_date = source["latest_update_date"];
	        this.missing_dlc = source["missing_dlc"];
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

export namespace settings {
	
	export class OrganizeOptions {
	    create_folder_per_game: boolean;
	    dlc_folder: string;
	    updates_folder: string;
	    rename_files: boolean;
	    delete_empty_folders: boolean;
	    delete_old_update_files: boolean;
	    folder_name_template: string;
	    switch_safe_file_names: boolean;
	    file_name_template: string;
	    process_when_missing_base_game: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OrganizeOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.create_folder_per_game = source["create_folder_per_game"];
	        this.dlc_folder = source["dlc_folder"];
	        this.updates_folder = source["updates_folder"];
	        this.rename_files = source["rename_files"];
	        this.delete_empty_folders = source["delete_empty_folders"];
	        this.delete_old_update_files = source["delete_old_update_files"];
	        this.folder_name_template = source["folder_name_template"];
	        this.switch_safe_file_names = source["switch_safe_file_names"];
	        this.file_name_template = source["file_name_template"];
	        this.process_when_missing_base_game = source["process_when_missing_base_game"];
	    }
	}
	export class AppSettings {
	    versions_json_url: string;
	    versions_etag: string;
	    titles_json_url: string;
	    titles_etag: string;
	    prod_keys: string;
	    folder: string;
	    scan_folders: string[];
	    gui: boolean;
	    debug: boolean;
	    check_for_missing_updates: boolean;
	    check_for_missing_dlc: boolean;
	    hide_missing_games: boolean;
	    hide_demo_games: boolean;
	    organize_options: OrganizeOptions;
	    scan_recursively: boolean;
	    gui_page_size: number;
	    ignore_dlc_updates: boolean;
	    ignore_dlc_title_ids: string[];
	    ignore_update_title_ids: string[];
	    ignore_file_types: string[];
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.versions_json_url = source["versions_json_url"];
	        this.versions_etag = source["versions_etag"];
	        this.titles_json_url = source["titles_json_url"];
	        this.titles_etag = source["titles_etag"];
	        this.prod_keys = source["prod_keys"];
	        this.folder = source["folder"];
	        this.scan_folders = source["scan_folders"];
	        this.gui = source["gui"];
	        this.debug = source["debug"];
	        this.check_for_missing_updates = source["check_for_missing_updates"];
	        this.check_for_missing_dlc = source["check_for_missing_dlc"];
	        this.hide_missing_games = source["hide_missing_games"];
	        this.hide_demo_games = source["hide_demo_games"];
	        this.organize_options = this.convertValues(source["organize_options"], OrganizeOptions);
	        this.scan_recursively = source["scan_recursively"];
	        this.gui_page_size = source["gui_page_size"];
	        this.ignore_dlc_updates = source["ignore_dlc_updates"];
	        this.ignore_dlc_title_ids = source["ignore_dlc_title_ids"];
	        this.ignore_update_title_ids = source["ignore_update_title_ids"];
	        this.ignore_file_types = source["ignore_file_types"];
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

export namespace switchfs {
	
	export class Content {
	    Text: string;
	    Type: string;
	    ID: string;
	    Size: string;
	    Hash: string;
	    KeyGeneration: string;
	
	    static createFrom(source: any = {}) {
	        return new Content(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Text = source["Text"];
	        this.Type = source["Type"];
	        this.ID = source["ID"];
	        this.Size = source["Size"];
	        this.Hash = source["Hash"];
	        this.KeyGeneration = source["KeyGeneration"];
	    }
	}
	export class NacpTitle {
	    Language: number;
	    Title: string;
	
	    static createFrom(source: any = {}) {
	        return new NacpTitle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Language = source["Language"];
	        this.Title = source["Title"];
	    }
	}
	export class Nacp {
	    TitleName: Record<string, NacpTitle>;
	    Isbn: string;
	    DisplayVersion: string;
	    SupportedLanguageFlag: number;
	
	    static createFrom(source: any = {}) {
	        return new Nacp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.TitleName = this.convertValues(source["TitleName"], NacpTitle, true);
	        this.Isbn = source["Isbn"];
	        this.DisplayVersion = source["DisplayVersion"];
	        this.SupportedLanguageFlag = source["SupportedLanguageFlag"];
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
	export class ContentMetaAttributes {
	    title_id: string;
	    version: number;
	    type: string;
	    Contents: Record<string, Content>;
	    Ncap?: Nacp;
	
	    static createFrom(source: any = {}) {
	        return new ContentMetaAttributes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title_id = source["title_id"];
	        this.version = source["version"];
	        this.type = source["type"];
	        this.Contents = this.convertValues(source["Contents"], Content, true);
	        this.Ncap = this.convertValues(source["Ncap"], Nacp);
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

