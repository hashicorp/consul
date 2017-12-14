// We know the master of the first set (pri=1), but not of the second.
var settings = {}
var rs1cfg = {_id: "rs1",
              members: [{_id: 1, host: "127.0.0.1:40011", priority: 1},
                        {_id: 2, host: "127.0.0.1:40012", priority: 0},
                        {_id: 3, host: "127.0.0.1:40013", priority: 0}]}
var rs2cfg = {_id: "rs2",
              members: [{_id: 1, host: "127.0.0.1:40021", priority: 1},
                        {_id: 2, host: "127.0.0.1:40022", priority: 1},
                        {_id: 3, host: "127.0.0.1:40023", priority: 0}]}
var rs3cfg = {_id: "rs3",
              members: [{_id: 1, host: "127.0.0.1:40031", priority: 1},
                        {_id: 2, host: "127.0.0.1:40032", priority: 1},
                        {_id: 3, host: "127.0.0.1:40033", priority: 1}],
              settings: settings}

for (var i = 0; i != 60; i++) {
	try {
		rs1a = new Mongo("127.0.0.1:40011").getDB("admin")
		rs2a = new Mongo("127.0.0.1:40021").getDB("admin")
		rs3a = new Mongo("127.0.0.1:40031").getDB("admin")
		rs3a.auth("root", "rapadura")
		db1 = new Mongo("127.0.0.1:40001").getDB("admin")
		db2 = new Mongo("127.0.0.1:40002").getDB("admin")
		break
	} catch(err) {
		print("Can't connect yet...")
	}
	sleep(1000)
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

for (var i = 0; i != 90; i++) {
    var count = countHealthy(rs1a) + countHealthy(rs2a) + countHealthy(rs3a)
    print("Replica sets have", count, "healthy nodes.")
    if (count == totalRSMembers) {
        quit(0)
    }
    sleep(1000)
}

print("Replica sets didn't sync up properly.")
quit(12)

// vim:ts=4:sw=4:et
