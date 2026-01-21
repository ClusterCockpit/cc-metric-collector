Name:           cc-metric-collector
Version:        %{VERS}
Release:        1%{?dist}
Summary:        Metric collection daemon from the ClusterCockpit suite

License:        MIT
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  go-toolset
BuildRequires:  systemd-rpm-macros
# for header downloads
BuildRequires:  wget
# Recommended when using the sysusers_create_package macro
Requires(pre): /usr/bin/systemd-sysusers

Provides:       %{name} = %{version}

%description
Metric collection daemon from the ClusterCockpit suite

%global debug_package %{nil}

%prep
%autosetup


%build
make


%install
install -Dpm 0750 %{name} %{buildroot}%{_bindir}/%{name}
install -Dpm 0600 example-configs/config.json %{buildroot}%{_sysconfdir}/%{name}/%{name}.json
install -Dpm 0600 example-configs/collectors.json %{buildroot}%{_sysconfdir}/%{name}/collectors.json
install -Dpm 0600 example-configs/sinks.json %{buildroot}%{_sysconfdir}/%{name}/sinks.json
install -Dpm 0600 example-configs/receivers.json %{buildroot}%{_sysconfdir}/%{name}/receivers.json
install -Dpm 0600 example-configs/router.json %{buildroot}%{_sysconfdir}/%{name}/router.json
install -Dpm 0644 scripts/%{name}.service %{buildroot}%{_unitdir}/%{name}.service
install -Dpm 0600 scripts/%{name}.config %{buildroot}%{_sysconfdir}/default/%{name}
install -Dpm 0644 scripts/%{name}.sysusers %{buildroot}%{_sysusersdir}/%{name}.conf


%check
# go test should be here... :)

%pre
%sysusers_create_package %{name} scripts/%{name}.sysusers

%post
%systemd_post %{name}.service

%preun
%systemd_preun %{name}.service

%files
# Binary
%attr(-,clustercockpit,clustercockpit) %{_bindir}/%{name}
# Config
%dir %{_sysconfdir}/%{name}
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/%{name}.json
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/collectors.json
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/sinks.json
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/receivers.json
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/router.json
# Systemd
%{_unitdir}/%{name}.service
%{_sysconfdir}/default/%{name}
%{_sysusersdir}/%{name}.conf

%changelog
* Thu Mar 03 2022 Thomas Gruber - 0.3
- Add clustercockpit user installation
* Mon Feb 14 2022 Thomas Gruber - 0.2
- Add component specific configuration files
- Add %attr to config files
* Mon Nov 22 2021 Thomas Gruber - 0.1
- Initial spec file
