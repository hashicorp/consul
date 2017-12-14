//var settings = {heartbeatSleep: 0.05, heartbeatTimeout: 0.5}
var settings = {};

// We know the master of the first set (pri=1), but not of the second.
var rs1cfg = {_id: "rs1",
              members: [{_id: 1, host: "127.0.0.1:40011", priority: 1, tags: {rs1: "a"}},
                        {_id: 2, host: "127.0.0.1:40012", priority: 0, tags: {rs1: "b"}},
                        {_id: 3, host: "127.0.0.1:40013", priority: 0, tags: {rs1: "c"}}],
              settings: settings}
var rs2cfg = {_id: "rs2",
              members: [{_id: 1, host: "127.0.0.1:40021", priority: 1, tags: {rs2: "a"}},
                        {_id: 2, host: "127.0.0.1:40022", priority: 1, tags: {rs2: "b"}},
                        {_id: 3, host: "127.0.0.1:40023", priority: 1, tags: {rs2: "c"}}],
              settings: settings}
var rs3cfg = {_id: "rs3",
              members: [{_id: 1, host: "127.0.0.1:40031", priority: 1, tags: {rs3: "a"}},
                        {_id: 2, host: "127.0.0.1:40032", priority: 1, tags: {rs3: "b"}},
                        {_id: 3, host: "127.0.0.1:40033", priority: 1, tags: {rs3: "c"}}],
              settings: settings}

for (var i = 0; i != 60; i++) {
	try {
		db1 = new Mongo("127.0.0.1:40001").getDB("admin")
		db2 = new Mongo("127.0.0.1:40002").getDB("admin")
		rs1a = new Mongo("127.0.0.1:40011").getDB("admin")
		rs2a = new Mongo("127.0.0.1:40021").getDB("admin")
		rs3a = new Mongo("127.0.0.1:40031").getDB("admin")
		break
	} catch(err) {
		print("Can't connect yet...")
	}
	sleep(1000)
}

function hasSSL() {
    return Boolean(db1.serverBuildInfo().OpenSSLVersion)
}

rs1a.runCommand({replSetInitiate: rs1cfg})
rs2a.runCommand({replSetInitiate: rs2cfg})
rs3a.runCommand({replSetInitiate: rs3cfg})

function configShards() {
    cfg1 = new Mongo("127.0.0.1:40201").getDB("admin")
    cfg1.runCommand({addshard: "127.0.0.1:40001"})
    cfg1.runCommand({addshard: "rs1/127.0.0.1:40011"})

    cfg2 = new Mongo("127.0.0.1:40202").getDB("admin")
    cfg2.runCommand({addshard: "rs2/127.0.0.1:40021"})

    cfg3 = new Mongo("127.0.0.1:40203").getDB("admin")
    cfg3.runCommand({addshard: "rs3/127.0.0.1:40031"})
}

function configAuth() {
    var addrs = ["127.0.0.1:40002", "127.0.0.1:40203", "127.0.0.1:40031"]
    if (hasSSL()) {
        addrs.push("127.0.0.1:40003")
    }
    for (var i in addrs) {
        print("Configuring auth for", addrs[i])
        var db = new Mongo(addrs[i]).getDB("admin")
        var v = db.serverBuildInfo().versionArray
        var timedOut = false
        if (v < [2, 5]) {
            db.addUser("root", "rapadura")
        } else {
            try {
                db.createUser({user: "root", pwd: "rapadura", roles: ["root"]})
            } catch (err) {
                // 3.2 consistently fails replication of creds on 40031 (config server) 
                print("createUser command returned an error: " + err)
                if (String(err).indexOf("timed out") >= 0) {
                    timedOut = true;
                }
            }
        }
        for (var i = 0; i < 60; i++) {
            var ok = db.auth("root", "rapadura")
            if (ok || !timedOut) {
                break
            }
            sleep(1000);
        }
        if (v >= [2, 6]) {
            db.createUser({user: "reader", pwd: "rapadura", roles: ["readAnyDatabase"]})
        } else if (v >= [2, 4]) {
            db.addUser({user: "reader", pwd: "rapadura", roles: ["readAnyDatabase"]})
        } else {
            db.addUser("reader", "rapadura", true)
        }
    }
}

function countHealthy(rs) {
    var status = rs.runCommand({replSetGetStatus: 1})
    var count = 0
    var primary = 0
    if (typeof status.members != "undefined") {
        for (var i = 0; i != status.members.length; i++) {
            var m = status.members[i]
            if (m.health == 1 && (m.state == 1 || m.state == 2)) {
                count += 1
                if (m.state == 1) {
                    primary = 1
                }
            }
        }
    }
    if (primary == 0) {
	    count = 0
    }
    return count
}

var totalRSMembers = rs1cfg.members.length + rs2cfg.members.length + rs3cfg.members.length

for (var i = 0; i != 60; i++) {
    var count = countHealthy(rs1a) + countHealthy(rs2a) + countHealthy(rs3a)
    print("Replica sets have", count, "healthy nodes.")
    if (count == totalRSMembers) {
        configShards()
        configAuth()
        quit(0)
    }
    sleep(1000)
}

print("Replica sets didn't sync up properly.")
quit(12)

// vim:ts=4:sw=4:et
