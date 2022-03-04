Name:           cc-metric-collector
Version:        %{VERS}
Release:        1%{?dist}
Summary:        Metric collection daemon from the ClusterCockpit suite

License:        MIT
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  go-toolset
BuildRequires:  systemd-rpm-macros
# for internal LIKWID installation
BuildRequires:  wget perl-Data-Dumper

Provides:       %{name} = %{version}

%description
Metric collection daemon from the ClusterCockpit suite

%global debug_package %{nil}

%prep
%autosetup


%build
make


%install
install -Dpm 0750 %{name} %{buildroot}%{_sbindir}/%{name}
install -Dpm 0600 config.json %{buildroot}%{_sysconfdir}/%{name}/%{name}.json
install -Dpm 0600 collectors.json %{buildroot}%{_sysconfdir}/%{name}/collectors.json
install -Dpm 0600 sinks.json %{buildroot}%{_sysconfdir}/%{name}/sinks.json
install -Dpm 0600 receivers.json %{buildroot}%{_sysconfdir}/%{name}/receivers.json
install -Dpm 0600 router.json %{buildroot}%{_sysconfdir}/%{name}/router.json
install -Dpm 0644 scripts/%{name}.service %{buildroot}%{_unitdir}/%{name}.service
install -Dpm 0600 scripts/%{name}.config %{buildroot}%{_sysconfdir}/default/%{name}
install -Dpm 0644 scripts/%{name}.sysusers %{buildroot}%{_sysusersdir}/%{name}.conf


%check
# go test should be here... :)

%pre
%sysusers_create_package scripts/%{name}.sysusers

%post
%systemd_post %{name}.service

%preun
%systemd_preun %{name}.service

%files
# Binary
%attr(-,clustercockpit,clustercockpit) %{_sbindir}/%{name}
# Configuration
%dir %{_sysconfdir}/%{name}
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/%{name}.json
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/collectors.json
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/sinks.json
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/receivers.json
%attr(0600,clustercockpit,clustercockpit) %config(noreplace) %{_sysconfdir}/%{name}/router.json
# Systemd
%{_sysusersdir}/%{name}.conf
%{_unitdir}/%{name}.service
%{_sysconfdir}/default/%{name}

%changelog
* Thu Mar 03 2022 Thomas Gruber - 0.3
- Add clustercockpit user installation
* Mon Feb 14 2022 Thomas Gruber - 0.2
- Add component specific configuration files
- Add %attr to config files
* Mon Nov 22 2021 Thomas Gruber - 0.1
- Initial spec file
