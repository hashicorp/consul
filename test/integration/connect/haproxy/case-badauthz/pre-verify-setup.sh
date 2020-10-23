wait_for_health_check_passing_state s1 primary
wait_for_health_check_passing_state s2 primary

# Setup deny intention
docker_consul primary intention create -deny s1 s2